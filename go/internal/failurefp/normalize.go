package failurefp

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

var (
	narrativeVerdictToken   = regexp.MustCompile(`narrative=(?:PASS|FAIL|WARN|SKIPPED)\b`)
	cycleAssignToken        = regexp.MustCompile(`\bcycle=\d+\b`)
	cycleULIDSuffixToken    = regexp.MustCompile(`(?i)\bcycle-\d+-[0-9A-HJKMNP-TV-Z]{26}\b`)
	cycleNumberToken        = regexp.MustCompile(`(?i)\bcycle[ -]\d+(?:-\d+)*`)
	findingCountToken       = regexp.MustCompile(`\b\d+ (findings?\b|finding\(s\))`)
	attemptDenominatorToken = regexp.MustCompile(`(?i)\b(attempt|retry)\s+\d+\s*(?:/|of)\s*\d+`)
	goTestDurationToken     = regexp.MustCompile(`\b\d+\.\d+s\b`)
)

func Normalize(reason string) string {
	reason = cyclestate.WithoutDetail(reason)
	reason = narrativeVerdictToken.ReplaceAllString(reason, "narrative=<verdict>")
	reason = cycleAssignToken.ReplaceAllString(reason, "cycle=N")
	reason = cycleULIDSuffixToken.ReplaceAllString(reason, "cycle-N-<ulid>")
	reason = cycleNumberToken.ReplaceAllString(reason, "cycle-N")
	reason = findingCountToken.ReplaceAllString(reason, "<n> ${1}")
	reason = attemptDenominatorToken.ReplaceAllString(reason, "${1} <n>/<n>")
	return strings.TrimSpace(goTestDurationToken.ReplaceAllString(reason, "<dur>"))
}
