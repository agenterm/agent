package gate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agenterm/cli/internal/relay"
)

// --- Hook protocol tests ---

func TestParseHookInput_Valid(t *testing.T) {
	input := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"},"tool_use_id":"t1"}`
	h := ParseHookInput([]byte(input))
	if h == nil {
		t.Fatal("expected non-nil HookInput")
	}
	if h.ToolName != "Bash" {
		t.Fatalf("expected ToolName=Bash, got %q", h.ToolName)
	}
}

func TestParseHookInput_PermissionRequest(t *testing.T) {
	input := `{"session_id":"s2","hook_event_name":"PermissionRequest","tool_name":"Bash","tool_input":{"command":"rm -rf /tmp"},"tool_use_id":"t2"}`
	h := ParseHookInput([]byte(input))
	if h == nil {
		t.Fatal("expected non-nil HookInput for PermissionRequest")
	}
	if h.HookEventName != "PermissionRequest" {
		t.Fatalf("expected HookEventName=PermissionRequest, got %q", h.HookEventName)
	}
	if h.ToolName != "Bash" {
		t.Fatalf("expected ToolName=Bash, got %q", h.ToolName)
	}
}

func TestParseHookInput_NotSupported(t *testing.T) {
	input := `{"hook_event_name":"PostToolUse","tool_name":"Bash"}`
	h := ParseHookInput([]byte(input))
	if h != nil {
		t.Fatal("expected nil for unsupported event")
	}
}

func TestParseHookInput_InvalidJSON(t *testing.T) {
	h := ParseHookInput([]byte("not json"))
	if h != nil {
		t.Fatal("expected nil for invalid JSON")
	}
}

func TestExtractCheckInput_Bash(t *testing.T) {
	h := &HookInput{
		ToolName:  "Bash",
		ToolInput: map[string]interface{}{"command": "git push --force origin main"},
	}
	got := ExtractCheckInput(h)
	if got != "git push --force origin main" {
		t.Fatalf("expected command string, got %q", got)
	}
}

func TestExtractCheckInput_NonBash(t *testing.T) {
	h := &HookInput{
		ToolName:  "Write",
		ToolInput: map[string]interface{}{"file_path": "/tmp/x.sql", "content": "DROP TABLE users"},
	}
	got := ExtractCheckInput(h)
	if got == "" {
		t.Fatal("expected non-empty serialized input")
	}
	// Should contain the DROP TABLE text
	if !contains(got, "DROP TABLE") {
		t.Fatalf("expected serialized input to contain DROP TABLE, got %q", got)
	}
}

func TestExtractCheckInput_BashMissingCommand(t *testing.T) {
	h := &HookInput{
		ToolName:  "Bash",
		ToolInput: map[string]interface{}{},
	}
	got := ExtractCheckInput(h)
	// Falls through to JSON serialization
	if got == "" {
		t.Fatal("expected non-empty fallback")
	}
}

func TestBuildHookOutput(t *testing.T) {
	out := BuildHookOutput("deny", "too dangerous")
	if out.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("expected hookEventName=PreToolUse, got %q", out.HookSpecificOutput.HookEventName)
	}
	if out.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("expected deny, got %q", out.HookSpecificOutput.PermissionDecision)
	}
	if out.HookSpecificOutput.PermissionDecisionReason != "too dangerous" {
		t.Fatalf("expected reason, got %q", out.HookSpecificOutput.PermissionDecisionReason)
	}

	// Verify JSON marshaling
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var roundtrip HookOutput
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if roundtrip.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatal("roundtrip failed")
	}
}

func TestBuildPermissionRequestOutput(t *testing.T) {
	out := BuildPermissionRequestOutput("allow", "approved via AgenTerm")
	if out.HookSpecificOutput.HookEventName != "PermissionRequest" {
		t.Fatalf("expected hookEventName=PermissionRequest, got %q", out.HookSpecificOutput.HookEventName)
	}
	if out.HookSpecificOutput.Decision.Behavior != "allow" {
		t.Fatalf("expected behavior=allow, got %q", out.HookSpecificOutput.Decision.Behavior)
	}
	if out.HookSpecificOutput.Decision.Message != "approved via AgenTerm" {
		t.Fatalf("expected message, got %q", out.HookSpecificOutput.Decision.Message)
	}

	// Verify JSON structure matches Claude Code PermissionRequest format
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	hso, ok := raw["hookSpecificOutput"].(map[string]interface{})
	if !ok {
		t.Fatal("missing hookSpecificOutput")
	}
	if hso["hookEventName"] != "PermissionRequest" {
		t.Fatalf("wrong hookEventName in JSON: %v", hso["hookEventName"])
	}
	decision, ok := hso["decision"].(map[string]interface{})
	if !ok {
		t.Fatal("missing decision object")
	}
	if decision["behavior"] != "allow" {
		t.Fatalf("wrong behavior in JSON: %v", decision["behavior"])
	}
}

func TestBuildPermissionRequestOutput_Deny(t *testing.T) {
	out := BuildPermissionRequestOutput("deny", "denied via AgenTerm")
	if out.HookSpecificOutput.Decision.Behavior != "deny" {
		t.Fatalf("expected behavior=deny, got %q", out.HookSpecificOutput.Decision.Behavior)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// mockRelayServer creates a test server that handles POST /proposals and GET /proposals/:id.
// finalStatus is the status returned on GET.
func mockRelayServer(finalStatus string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "test-123",
				"status": "pending",
			})
		case "GET":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "test-123",
				"status": finalStatus,
			})
		}
	}))
}

func newTestClient(serverURL string) *relay.Client {
	return &relay.Client{
		BaseURL:    serverURL,
		PushKey:    "test-key",
		HTTPClient: http.DefaultClient,
	}
}

func TestRunGate_NoMatch(t *testing.T) {
	// TC-G01: safe input, no match
	client := newTestClient("http://unused")
	rules := DefaultRules()

	result, err := RunGate(client, "ls", rules, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NeedsApproval {
		t.Fatal("expected NeedsApproval=false")
	}
	if result.Decision != "allow" {
		t.Fatalf("expected Decision=allow, got %q", result.Decision)
	}
}

func TestRunGate_Approved(t *testing.T) {
	// TC-G02: dangerous input + approved
	srv := mockRelayServer("approved")
	defer srv.Close()

	client := newTestClient(srv.URL)
	rules := DefaultRules()

	result, err := RunGate(client, "rm -rf /", rules, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NeedsApproval {
		t.Fatal("expected NeedsApproval=true")
	}
	if result.Decision != "approved" {
		t.Fatalf("expected Decision=approved, got %q", result.Decision)
	}
}

func TestRunGate_Denied(t *testing.T) {
	// TC-G03: dangerous input + denied
	srv := mockRelayServer("denied")
	defer srv.Close()

	client := newTestClient(srv.URL)
	rules := DefaultRules()

	result, err := RunGate(client, "rm -rf /", rules, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != "denied" {
		t.Fatalf("expected Decision=denied, got %q", result.Decision)
	}
}

func TestRunGate_Expired(t *testing.T) {
	// TC-G04: dangerous input + expired → maps to denied
	srv := mockRelayServer("expired")
	defer srv.Close()

	client := newTestClient(srv.URL)
	rules := DefaultRules()

	result, err := RunGate(client, "rm -rf /", rules, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != "denied" {
		t.Fatalf("expected Decision=denied (expired→denied), got %q", result.Decision)
	}
}
