package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/agenterm/cli/internal/agent"
	"github.com/agenterm/cli/internal/config"
	"github.com/agenterm/cli/internal/gate"
	"github.com/agenterm/cli/internal/relay"
)

func runGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	timeout := fs.Int("timeout", 60, "approval timeout in seconds")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	// APN enforces a minimum expires_in of 60s; clamp to avoid silent mismatch.
	if *timeout < 60 {
		*timeout = 60
	}

	// If positional args provided, use legacy mode (raw text).
	if remaining := fs.Args(); len(remaining) > 0 {
		return runGateLegacy(strings.Join(remaining, " "), *timeout)
	}

	// Otherwise, read stdin and auto-detect hook JSON vs raw text.
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		return 2
	}

	if hookInput := agent.ParseHookInput(data); hookInput != nil {
		return runGateHook(hookInput, *timeout)
	}

	// Legacy mode: treat stdin as raw text.
	input := strings.TrimSpace(string(data))
	if input == "" {
		fmt.Fprintln(os.Stderr, "Usage: agenterm gate <tool_input>")
		return 2
	}
	return runGateLegacy(input, *timeout)
}

// runGateHook dispatches any hook event via the unified /hooks endpoint.
func runGateHook(hookInput *agent.HookInput, timeout int) int {
	// For observability events without config, just exit silently.
	if agent.IsObservabilityEvent(hookInput.HookEventName) {
		cfg, err := config.Load()
		if err != nil {
			return 0
		}
		client := relay.NewClient(cfg)
		_, _ = client.ForwardHook(hookInput.RawJSON)
		return 0
	}

	// Decision event: forward to APN and wait for response.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: config not found, falling back to local prompt\n")
		out := agent.OutputterForEvent(hookInput.HookEventName)
		fallbackPrompt := fmt.Sprintf("[%s] %s", hookInput.HookEventName, hookInput.ToolName)
		if askLocallyInTerminal(fallbackPrompt) {
			return outputDecision(out.Allow("approved locally via AgenTerm (no config)"))
		}
		return outputDecision(out.Deny("denied locally via AgenTerm (no config)"))
	}

	client := relay.NewClient(cfg)
	hookResp, err := client.ForwardHook(hookInput.RawJSON)

	out := agent.OutputterForEvent(hookInput.HookEventName)
	fallbackPrompt := fmt.Sprintf("[%s] %s", hookInput.HookEventName, hookInput.ToolName)

	if errors.Is(err, relay.ErrGateDisabled) {
		fmt.Fprintf(os.Stderr, "gate: takeover not enabled, falling back to local prompt\n")
		if askLocallyInTerminal(fallbackPrompt) {
			return outputDecision(out.Allow("approved locally via AgenTerm"))
		}
		return outputDecision(out.Deny("denied locally via AgenTerm"))
	}
	if errors.Is(err, relay.ErrPushKeyExpired) {
		fmt.Fprintf(os.Stderr, "gate: push key expired or invalid — copy a fresh key from the AgenTerm app\n")
		fmt.Fprintf(os.Stderr, "gate: falling back to local prompt\n")
		if askLocallyInTerminal(fallbackPrompt) {
			outputJSON(out.Allow("approved locally via AgenTerm (push key expired)"))
		} else {
			outputJSON(out.Deny("denied locally via AgenTerm (push key expired)"))
		}
		return 0
	}
	if errors.Is(err, relay.ErrRateLimited) {
		fmt.Fprintf(os.Stderr, "gate: rate limited, denying for safety\n")
		return outputDecision(out.Deny("rate limited by AgenTerm relay"))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: error: %v\n", err)
		fmt.Fprintf(os.Stderr, "gate: falling back to local prompt\n")
		if askLocallyInTerminal(fallbackPrompt) {
			return outputDecision(out.Allow("approved locally via AgenTerm (relay error)"))
		}
		return outputDecision(out.Deny("denied locally via AgenTerm (relay error)"))
	}

	if hookResp.Mode == "observability" {
		return 0
	}

	// Decision mode: long-poll for proposal resolution.
	proposal, err := client.WaitForProposal(hookResp.Proposal.ID, time.Duration(timeout)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: error waiting: %v\n", err)
		fmt.Fprintf(os.Stderr, "gate: falling back to local prompt\n")
		if askLocallyInTerminal(fallbackPrompt) {
			return outputDecision(out.Allow("approved locally via AgenTerm (relay error)"))
		}
		return outputDecision(out.Deny("denied locally via AgenTerm (relay error)"))
	}

	switch gate.NormalizeStatus(proposal.Status) {
	case "approved":
		return outputDecision(out.Allow("approved via AgenTerm"))
	default:
		return outputDecision(out.Deny("denied via AgenTerm"))
	}
}

// outputDecision writes the outputter result to stdout and returns the appropriate exit code.
func outputDecision(result interface{}) int {
	if result == nil {
		return 0
	}

	// ExitCodeDeny means deny via exit code 2 + stderr.
	if deny, ok := result.(*agent.ExitCodeDeny); ok {
		fmt.Fprintln(os.Stderr, deny.Reason)
		return 2
	}

	outputJSON(result)
	return 0
}

// runGateLegacy handles the original gate behavior (raw text input).
func runGateLegacy(input string, timeout int) int {
	rules := gate.DefaultRules()
	matched, rule := gate.MatchesAny(input, rules)
	if !matched {
		fmt.Fprintln(os.Stderr, "no dangerous pattern matched, allowing")
		return 0
	}

	fmt.Fprintf(os.Stderr, "matched rule: %s\n", rule.Description)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: config not found, falling back to local prompt\n")
		if askLocallyInTerminal(input) {
			return 0
		}
		return 1
	}

	client := relay.NewClient(cfg)
	result, err := gate.RunGate(client, input, rules, time.Duration(timeout)*time.Second)
	if errors.Is(err, relay.ErrGateDisabled) {
		fmt.Fprintf(os.Stderr, "gate: takeover not enabled, falling back to local prompt\n")
		if askLocallyInTerminal(input) {
			return 0
		} else {
			return 1
		}
	}
	if errors.Is(err, relay.ErrPushKeyExpired) {
		fmt.Fprintf(os.Stderr, "gate: push key expired or invalid — copy a fresh key from the AgenTerm app\n")
		fmt.Fprintf(os.Stderr, "gate: falling back to local prompt\n")
		if askLocallyInTerminal(input) {
			return 0
		}
		return 1
	}
	if errors.Is(err, relay.ErrRateLimited) {
		fmt.Fprintf(os.Stderr, "rate limited by relay server, denying for safety\n")
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: error: %v\n", err)
		fmt.Fprintf(os.Stderr, "gate: falling back to local prompt\n")
		if askLocallyInTerminal(input) {
			return 0
		}
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding output: %v\n", err)
		return 2
	}

	fmt.Fprintf(os.Stderr, "decision: %s\n", result.Decision)

	switch result.Decision {
	case "approved":
		return 0
	case "denied":
		return 1
	default:
		return 2
	}
}
