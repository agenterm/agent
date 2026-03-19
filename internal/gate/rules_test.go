package gate

import (
	"strings"
	"testing"
)

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()

	tests := []struct {
		name        string
		input       string
		wantMatch   bool
		wantDescSub string // substring of Description, empty if no match expected
	}{
		{"TC-R01: rm -rf matches", "rm -rf /tmp", true, "Recursive delete"},
		{"TC-R02: rm -r matches", "rm -r ./old", true, "Recursive delete"},
		{"TC-R03: rm without -r no match", "rm file.txt", false, ""},
		{"TC-R04: git push --force matches", "git push --force origin main", true, "Force push"},
		{"TC-R05: git push feature no match", "git push origin feature/foo", false, ""},
		{"TC-R06: git reset --hard matches", "git reset --hard HEAD~1", true, "Hard reset"},
		{"TC-R07: DROP TABLE matches", "DROP TABLE users", true, "Drop database table"},
		{"TC-R08: drop table lowercase matches", "drop table users", true, "Drop database table"},
		{"TC-R09: ls safe no match", "ls -la /tmp", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, rule := MatchesAny(tt.input, rules)
			if matched != tt.wantMatch {
				t.Fatalf("MatchesAny(%q) matched=%v, want %v", tt.input, matched, tt.wantMatch)
			}
			if tt.wantMatch && !strings.Contains(rule.Description, tt.wantDescSub) {
				t.Fatalf("MatchesAny(%q) description=%q, want substring %q", tt.input, rule.Description, tt.wantDescSub)
			}
		})
	}
}
