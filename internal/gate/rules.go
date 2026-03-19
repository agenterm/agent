package gate

import "regexp"

// Rule defines a pattern that triggers approval.
type Rule struct {
	Pattern     string
	Description string
	re          *regexp.Regexp
}

// DefaultRules returns the built-in set of dangerous operation rules.
func DefaultRules() []Rule {
	rules := []Rule{
		{Pattern: `rm\s+-(rf|fr|r)\b`, Description: "Recursive delete"},
		{Pattern: `git\s+push\s+--force`, Description: "Force push"},
		{Pattern: `git\s+push\s+.*main`, Description: "Push to main branch"},
		{Pattern: `git\s+reset\s+--hard`, Description: "Hard reset"},
		{Pattern: `(?i)DROP\s+TABLE`, Description: "Drop database table"},
		{Pattern: `(?i)DELETE\s+FROM`, Description: "Delete from database"},
		{Pattern: `chmod\s+777`, Description: "Set overly permissive file mode"},
		{Pattern: `kill\s+-9`, Description: "Force kill process"},
		{Pattern: `[^2]>\s*/`, Description: "Overwrite file via redirect"},
	}
	for i := range rules {
		rules[i].re = regexp.MustCompile(rules[i].Pattern)
	}
	return rules
}

// MatchesAny checks if input matches any rule. Returns the first matched rule.
func MatchesAny(input string, rules []Rule) (bool, Rule) {
	for _, r := range rules {
		re := r.re
		if re == nil {
			re = regexp.MustCompile(r.Pattern)
		}
		if re.MatchString(input) {
			return true, r
		}
	}
	return false, Rule{}
}
