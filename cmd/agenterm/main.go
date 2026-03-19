package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/agenterm/cli/internal/config"
	"github.com/agenterm/cli/internal/gate"
	"github.com/agenterm/cli/internal/hook"
	"github.com/agenterm/cli/internal/relay"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
		os.Exit(0)
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "propose":
		os.Exit(runPropose(os.Args[2:]))
	case "proposal":
		os.Exit(runProposal(os.Args[2:]))
	case "gate":
		os.Exit(runGate(os.Args[2:]))
	case "hook":
		os.Exit(runHook(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: agenterm <command> [args]")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  init      [--push-key KEY] [--relay-url URL]")
	fmt.Fprintln(os.Stderr, "  version   Print version and exit")
	fmt.Fprintln(os.Stderr, "  propose   --title \"...\" --body \"...\" [--wait] [--timeout 60]")
	fmt.Fprintln(os.Stderr, "  proposal  status <proposal_id>")
	fmt.Fprintln(os.Stderr, "  hook      install [claude|gemini]")
	fmt.Fprintln(os.Stderr, "  hook      uninstall [claude|gemini]")
	fmt.Fprintln(os.Stderr, "  gate      (internal) used by hooks, not for direct use")
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	pushKey := fs.String("push-key", "", "push key from AgenTerm app")
	relayURL := fs.String("relay-url", "", "relay server URL")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	cfg := &config.Config{
		RelayURL: *relayURL,
		PushKey:  *pushKey,
	}

	// Interactive mode when --push-key is not provided
	if cfg.PushKey == "" {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("AgenTerm Agent Setup")
		fmt.Println()

		fmt.Printf("Relay URL (default: %s): ", config.DefaultRelayURL)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line != "" {
			cfg.RelayURL = line
		}

		fmt.Print("Push Key (from AgenTerm app > Account Details): ")
		line, _ = reader.ReadString('\n')
		cfg.PushKey = strings.TrimSpace(line)

		if cfg.PushKey == "" {
			fmt.Fprintln(os.Stderr, "error: push key is required")
			return 1
		}
	}

	if cfg.RelayURL == "" {
		cfg.RelayURL = config.DefaultRelayURL
	}

	fmt.Fprint(os.Stderr, "Validating push key... ")
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "\nerror: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "ok")

	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		return 1
	}

	p, _ := config.ConfigPath()
	fmt.Fprintf(os.Stderr, "Configuration saved to %s\n", p)
	return 0
}

func runPropose(args []string) int {
	fs := flag.NewFlagSet("propose", flag.ContinueOnError)
	title := fs.String("title", "", "proposal title")
	body := fs.String("body", "", "proposal body")
	wait := fs.Bool("wait", true, "wait for proposal result")
	timeout := fs.Int("timeout", 60, "wait timeout in seconds")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	// Allow title as positional arg for convenience.
	if *title == "" {
		if remaining := fs.Args(); len(remaining) > 0 {
			*title = remaining[0]
		}
	}

	if *title == "" || *body == "" {
		fmt.Fprintln(os.Stderr, "Usage: agenterm propose --title \"...\" --body \"...\" [--wait] [--timeout 60]")
		fmt.Fprintln(os.Stderr, "       agenterm propose <title> --body \"...\"  (title as positional arg, flags must come first)")
		return 2
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 2
	}

	client := relay.NewClient(cfg)

	proposal, err := client.CreateProposal("approval", *title, *body, relay.WithExpiresIn(*timeout))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating proposal: %v\n", err)
		return 2
	}

	if !*wait {
		return outputProposal(proposal)
	}

	proposal, err = client.WaitForProposal(proposal.ID, time.Duration(*timeout)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error waiting for proposal: %v\n", err)
		return 2
	}

	return outputProposal(proposal)
}

func runProposal(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: agenterm proposal status <proposal_id>")
		return 2
	}

	switch args[0] {
	case "status":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: agenterm proposal status <proposal_id>")
			return 2
		}
		return runProposalStatus(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown proposal command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: agenterm proposal status <proposal_id>")
		return 2
	}
}

func runProposalStatus(id string) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 2
	}

	client := relay.NewClient(cfg)

	proposal, err := client.GetProposal(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting proposal: %v\n", err)
		return 2
	}

	return outputProposal(proposal)
}

func runHook(args []string) int {
	if len(args) < 1 {
		printHookUsage()
		return 2
	}

	target := "all"
	if len(args) >= 2 {
		target = args[1]
	}

	switch args[0] {
	case "install":
		return runHookInstall(target)
	case "uninstall":
		return runHookUninstall(target)
	default:
		fmt.Fprintf(os.Stderr, "unknown hook command: %s\n", args[0])
		printHookUsage()
		return 2
	}
}

func printHookUsage() {
	fmt.Fprintln(os.Stderr, "Usage: agenterm hook <install|uninstall> [claude|gemini]")
	fmt.Fprintln(os.Stderr, "  If no target is specified, applies to all supported agents.")
}

func runHookInstall(target string) int {
	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine binary path: %v\n", err)
		return 1
	}

	type hookTarget struct {
		name         string
		install      func(string, string) error
		settingsPath func() (string, error)
		hookName     string
	}

	targets := []hookTarget{
		{"claude", hook.Install, hook.SettingsPath, "PermissionRequest"},
		{"gemini", hook.InstallGemini, hook.GeminiSettingsPath, "BeforeTool"},
	}

	matched := false
	exitCode := 0
	for _, t := range targets {
		if target != "all" && target != t.name {
			continue
		}
		matched = true
		if err := t.install(binaryPath, ""); err != nil {
			if errors.Is(err, hook.ErrAlreadyInstalled) {
				fmt.Fprintf(os.Stderr, "[%s] hook already installed\n", t.name)
				continue
			}
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", t.name, err)
			exitCode = 1
			continue
		}
		settingsPath, _ := t.settingsPath()
		fmt.Fprintf(os.Stderr, "[%s] Installed %s hook in %s\n", t.name, t.hookName, settingsPath)
	}

	if !matched {
		fmt.Fprintf(os.Stderr, "unknown target: %s (use claude, gemini, or omit for all)\n", target)
		return 2
	}
	return exitCode
}

func runHookUninstall(target string) int {
	type hookTarget struct {
		name         string
		uninstall    func(string) error
		settingsPath func() (string, error)
	}

	targets := []hookTarget{
		{"claude", hook.Uninstall, hook.SettingsPath},
		{"gemini", hook.UninstallGemini, hook.GeminiSettingsPath},
	}

	matched := false
	exitCode := 0
	for _, t := range targets {
		if target != "all" && target != t.name {
			continue
		}
		matched = true
		if err := t.uninstall(""); err != nil {
			if errors.Is(err, hook.ErrNotInstalled) {
				fmt.Fprintf(os.Stderr, "[%s] hook not found\n", t.name)
				continue
			}
			fmt.Fprintf(os.Stderr, "[%s] error: %v\n", t.name, err)
			exitCode = 1
			continue
		}
		settingsPath, _ := t.settingsPath()
		fmt.Fprintf(os.Stderr, "[%s] Uninstalled hook from %s\n", t.name, settingsPath)
	}

	if !matched {
		fmt.Fprintf(os.Stderr, "unknown target: %s (use claude, gemini, or omit for all)\n", target)
		return 2
	}
	return exitCode
}

func runGate(args []string) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	timeout := fs.Int("timeout", 60, "approval timeout in seconds")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
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

	if hookInput := gate.ParseHookInput(data); hookInput != nil {
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

// runGateHook handles Claude Code hook protocol (PreToolUse and PermissionRequest).
// Reads hook JSON, checks rules, sends approval proposal if needed,
// and outputs hook-compatible JSON. Always exits 0 (decision in JSON) or 2 (hard error).
func runGateHook(hookInput *gate.HookInput, timeout int) int {
	switch hookInput.HookEventName {
	case "PermissionRequest":
		return runGatePermissionRequest(hookInput, timeout)
	case "BeforeTool":
		return runGateBeforeTool(hookInput, timeout)
	default:
		return runGatePreToolUse(hookInput, timeout)
	}
}

// runGateBeforeTool handles Gemini CLI BeforeTool hooks: check rules, propose if dangerous.
func runGateBeforeTool(hookInput *gate.HookInput, timeout int) int {
	return runGateToolCheck(hookInput, timeout, geminiOutputter{})
}

// runGatePreToolUse handles PreToolUse hooks: check rules, propose if dangerous.
func runGatePreToolUse(hookInput *gate.HookInput, timeout int) int {
	return runGateToolCheck(hookInput, timeout, claudeOutputter{})
}

// gateOutputter abstracts the output format for different hook protocols.
type gateOutputter interface {
	allow(reason string)
	deny(reason string)
}

type geminiOutputter struct{}

func (geminiOutputter) allow(_ string) {
	outputJSON(&gate.GeminiHookOutput{Decision: "allow"})
}
func (geminiOutputter) deny(reason string) {
	outputJSON(&gate.GeminiHookOutput{Decision: "deny", Reason: reason})
}

type claudeOutputter struct{}

func (claudeOutputter) allow(reason string) {
	outputJSON(gate.BuildHookOutput("allow", reason))
}
func (claudeOutputter) deny(reason string) {
	outputJSON(gate.BuildHookOutput("deny", reason))
}

// runGateToolCheck is the shared gate logic for both PreToolUse and BeforeTool hooks.
func runGateToolCheck(hookInput *gate.HookInput, timeout int, out gateOutputter) int {
	input := gate.ExtractCheckInput(hookInput)
	if input == "" {
		out.allow("no input to check")
		return 0
	}

	rules := gate.DefaultRules()
	matched, rule := gate.MatchesAny(input, rules)
	if !matched {
		out.allow("no dangerous pattern matched")
		return 0
	}

	fmt.Fprintf(os.Stderr, "matched rule: %s\n", rule.Description)

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: config not found, blocking dangerous operation: %s\n", rule.Description)
		return 2
	}

	client := relay.NewClient(cfg)
	result, err := gate.RunGate(client, input, rules, time.Duration(timeout)*time.Second)
	if errors.Is(err, relay.ErrGateDisabled) {
		fmt.Fprintf(os.Stderr, "gate: takeover not enabled, falling back to local prompt\n")
		if askLocallyInTerminal(input) {
			out.allow("approved locally via AgenTerm")
		} else {
			out.deny("denied locally via AgenTerm")
		}
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: approval error: %v\n", err)
		return 2
	}

	switch result.Decision {
	case "approved", "allow":
		out.allow("approved via AgenTerm")
	default:
		out.deny(fmt.Sprintf("denied via AgenTerm: %s", rule.Description))
	}
	return 0
}

// runGatePermissionRequest handles PermissionRequest hooks.
// Claude Code has already determined this action needs permission — no rule matching needed.
// Directly submit a proposal and wait for approval.
func runGatePermissionRequest(hookInput *gate.HookInput, timeout int) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: config not found, blocking permission request\n")
		return 2
	}

	// Build a meaningful title and body for the proposal.
	title := hookInput.ToolName
	if title == "" {
		title = "Permission Request"
	}
	body := ""
	if len(hookInput.ToolInput) > 0 {
		data, err := json.Marshal(hookInput.ToolInput)
		if err == nil {
			body = string(data)
		}
	}

	client := relay.NewClient(cfg)
	proposal, err := client.CreateProposal("approval", title, body, relay.WithExpiresIn(timeout))
	if errors.Is(err, relay.ErrGateDisabled) {
		fmt.Fprintf(os.Stderr, "gate: takeover not enabled, falling back to local prompt\n")
		if askLocallyInTerminal(fmt.Sprintf("Tool: %s\nInput: %s", title, body)) {
			outputJSON(gate.BuildPermissionRequestOutput("allow", "approved locally via AgenTerm"))
		} else {
			outputJSON(gate.BuildPermissionRequestOutput("deny", "denied locally via AgenTerm"))
		}
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: error creating proposal: %v\n", err)
		return 2
	}

	proposal, err = client.WaitForProposal(proposal.ID, time.Duration(timeout)*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gate: error waiting for approval: %v\n", err)
		return 2
	}

	switch proposal.Status {
	case "approved", "remembered":
		outputJSON(gate.BuildPermissionRequestOutput("allow", "approved via AgenTerm"))
	default:
		outputJSON(gate.BuildPermissionRequestOutput("deny", "denied via AgenTerm"))
	}
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
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 2
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
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
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

func outputJSON(v interface{}) {
	json.NewEncoder(os.Stdout).Encode(v)
}

// outputProposal prints the proposal as JSON to stdout and returns the exit code.
func outputProposal(p *relay.Proposal) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		fmt.Fprintf(os.Stderr, "error encoding output: %v\n", err)
		return 2
	}

	switch p.Status {
	case "approved", "remembered", "pending":
		return 0
	case "denied", "dismissed":
		return 1
	default:
		return 2
	}
}

// askLocallyInTerminal opens the user's terminal directly (/dev/tty) to prompt for approval
// bypassing stdin/stdout which are used for Hook JSON communication.
func askLocallyInTerminal(cmd string) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open terminal for local prompt, defaulting to deny: %v\n", err)
		return false
	}
	defer tty.Close()

	fmt.Fprintf(tty, "\n🛡️  [AgenTerm Local Security Approval]\n")
	fmt.Fprintf(tty, "Agent is requesting to execute the following command/tool:\n> %s\n\n", cmd)
	fmt.Fprintf(tty, "Allow execution? [y/N]: ")

	reader := bufio.NewReader(tty)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	return answer == "y" || answer == "yes"
}
