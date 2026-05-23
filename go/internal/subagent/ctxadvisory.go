package subagent

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// CheckCtxAdvisoryResult carries the outcome of the context-tokens advisory
// check. Emit is true iff an advisory line should be printed.
type CheckCtxAdvisoryResult struct {
	Emit      bool
	Threshold int    // resolved value from profile.context_clear_trigger_tokens, 0 if absent
	Message   string // advisory line; empty unless Emit is true
}

// CheckCtxAdvisory parses the profile JSON and decides whether to emit an
// advisory when the test-agent's current context size exceeds the profile's
// declared threshold. Mirrors cmd_check_ctx_advisory at subagent-run.sh:605.
//
// Returns (result, error). Error is non-nil only when the profile file
// cannot be read; the bash version WARNs and exit 0s when the profile is
// missing — we return (Emit=false, err) so the CLI can decide whether to
// surface the WARN.
func CheckCtxAdvisory(profilePath string, tokens int) (CheckCtxAdvisoryResult, error) {
	body, err := os.ReadFile(profilePath)
	if err != nil {
		return CheckCtxAdvisoryResult{}, fmt.Errorf("subagent/ctxadvisory: read profile %s: %w", profilePath, err)
	}
	threshold, ok := extractInt(string(body), "context_clear_trigger_tokens")
	if !ok {
		// Profile doesn't declare the trigger; bash exit 0 without printing.
		return CheckCtxAdvisoryResult{Emit: false}, nil
	}
	res := CheckCtxAdvisoryResult{Threshold: threshold}
	if tokens > threshold {
		res.Emit = true
		res.Message = fmt.Sprintf(
			"test-agent context at ~%d tokens; profile threshold=%d (context_clear_trigger_tokens). Agent should apply Tool-Result Hygiene before further tool calls.",
			tokens, threshold,
		)
	}
	return res, nil
}

// extractInt looks for `"<field>": <int>` in body and returns (value, true)
// on match. Kept jq-free for the same reason as extractProfileString.
func extractInt(body, field string) (int, bool) {
	re := regexp.MustCompile(fmt.Sprintf(`"%s"\s*:\s*([0-9]+)`, regexp.QuoteMeta(field)))
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}
