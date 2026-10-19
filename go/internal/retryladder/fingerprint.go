package retryladder

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

var (
	violationCodeToken      = regexp.MustCompile(`\[([A-Za-z0-9_.:-]+)\]`)
	narrativeVerdictToken   = regexp.MustCompile(`narrative=(?:PASS|FAIL|WARN|SKIPPED)\b`)
	goTestDurationToken     = regexp.MustCompile(`\b\d+\.\d+s\b`)
	cycleNumberToken        = regexp.MustCompile(`(?i)\bcycle[ -]\d+(?:-\d+)*`)
	attemptDenominatorToken = regexp.MustCompile(`(?i)\b(attempt|retry)\s+\d+\s*(?:/|of)\s*\d+`)
)

func Fingerprint(reason string) string {
	reason = cyclestate.WithoutDetail(reason)
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	if codes := violationCodes(reason); len(codes) > 0 {
		return "codes:" + strings.Join(codes, ",")
	}
	sum := sha256.Sum256([]byte(normalize(reason)))
	return "text:" + hex.EncodeToString(sum[:])[:12]
}

func violationCodes(reason string) []string {
	var codes []string
	for _, m := range violationCodeToken.FindAllStringSubmatch(reason, -1) {
		codes = append(codes, m[1])
	}
	slices.Sort(codes)
	return slices.Compact(codes)
}

func normalize(reason string) string {
	reason = narrativeVerdictToken.ReplaceAllString(reason, "narrative=<verdict>")
	reason = cycleNumberToken.ReplaceAllString(reason, "cycle-N")
	reason = attemptDenominatorToken.ReplaceAllString(reason, "${1} <n>/<n>")
	return strings.TrimSpace(goTestDurationToken.ReplaceAllString(reason, "<dur>"))
}
