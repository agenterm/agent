package gate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/agenterm/cli/internal/relay"
)

// GateResult holds the outcome of a gate check.
type GateResult struct {
	NeedsApproval bool   `json:"needs_approval"`
	Rule          string `json:"rule,omitempty"`
	Decision      string `json:"decision"`
}

// HookInput represents the JSON Claude Code passes to PreToolUse hooks via stdin.
type HookInput struct {
	SessionID     string                 `json:"session_id"`
	HookEventName string                 `json:"hook_event_name"`
	ToolName      string                 `json:"tool_name"`
	ToolInput     map[string]interface{} `json:"tool_input"`
	ToolUseID     string                 `json:"tool_use_id"`
}

// HookOutput represents the JSON response for Claude Code PreToolUse hooks.
type HookOutput struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

// HookSpecificOutput contains the permission decision for the hook.
type HookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// ParseHookInput attempts to parse raw bytes as a Claude Code or Gemini hook input.
// Returns nil if the input is not valid hook JSON or not a supported event.
// Supported events: PreToolUse, PermissionRequest, BeforeTool.
func ParseHookInput(data []byte) *HookInput {
	var h HookInput
	if err := json.Unmarshal(data, &h); err != nil {
		return nil
	}
	if h.HookEventName != "PreToolUse" && h.HookEventName != "PermissionRequest" && h.HookEventName != "BeforeTool" {
		return nil
	}
	return &h
}

// ExtractCheckInput extracts the string to check against rules from a hook input.
// For Bash tools, extracts the command. For others, serializes tool_input.
func ExtractCheckInput(h *HookInput) string {
	if h.ToolName == "Bash" {
		if cmd, ok := h.ToolInput["command"].(string); ok {
			return cmd
		}
	}
	// For non-Bash tools, serialize all tool_input values to catch patterns
	// like DROP TABLE in Write/Edit content.
	data, err := json.Marshal(h.ToolInput)
	if err != nil {
		return ""
	}
	return string(data)
}

// BuildHookOutput creates a HookOutput for PreToolUse with the given decision and reason.
func BuildHookOutput(decision, reason string) *HookOutput {
	return &HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       decision,
			PermissionDecisionReason: reason,
		},
	}
}

// PermissionRequestOutput represents the JSON response for Claude Code PermissionRequest hooks.
type PermissionRequestOutput struct {
	HookSpecificOutput PermissionRequestSpecificOutput `json:"hookSpecificOutput"`
}

// PermissionRequestSpecificOutput contains the decision for a PermissionRequest hook.
type PermissionRequestSpecificOutput struct {
	HookEventName string                    `json:"hookEventName"`
	Decision      PermissionRequestDecision `json:"decision"`
}

// PermissionRequestDecision holds the behavior and message for a PermissionRequest response.
type PermissionRequestDecision struct {
	Behavior string `json:"behavior"`
	Message  string `json:"message,omitempty"`
}

// BuildPermissionRequestOutput creates a PermissionRequestOutput with the given behavior and message.
func BuildPermissionRequestOutput(behavior, message string) *PermissionRequestOutput {
	return &PermissionRequestOutput{
		HookSpecificOutput: PermissionRequestSpecificOutput{
			HookEventName: "PermissionRequest",
			Decision: PermissionRequestDecision{
				Behavior: behavior,
				Message:  message,
			},
		},
	}
}

// GeminiHookOutput represents the JSON response for Gemini CLI BeforeTool hooks.
type GeminiHookOutput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// RunGate checks input against rules and, if matched, creates an approval proposal and waits.
func RunGate(client *relay.Client, input string, rules []Rule, timeout time.Duration) (*GateResult, error) {
	matched, rule := MatchesAny(input, rules)
	if !matched {
		return &GateResult{NeedsApproval: false, Decision: "allow"}, nil
	}

	proposal, err := client.CreateProposal("approval", rule.Description, input, relay.WithExpiresIn(int(timeout.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("creating approval proposal: %w", err)
	}

	proposal, err = client.WaitForProposal(proposal.ID, timeout)
	if err != nil {
		return nil, fmt.Errorf("waiting for approval: %w", err)
	}

	decision := proposal.Status
	if decision == "remembered" {
		decision = "approved"
	} else if decision == "dismissed" || decision == "expired" {
		decision = "denied"
	}

	return &GateResult{
		NeedsApproval: true,
		Rule:          rule.Description,
		Decision:      decision,
	}, nil
}
