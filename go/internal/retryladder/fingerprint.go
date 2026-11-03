package retryladder

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurefp"
)

var violationCodeToken = regexp.MustCompile(`\[([A-Za-z0-9_.:-]+)\]`)

func Fingerprint(reason string) string {
	reason = cyclestate.WithoutDetail(reason)
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	if codes := violationCodes(reason); len(codes) > 0 {
		return "codes:" + strings.Join(codes, ",")
	}
	sum := sha256.Sum256([]byte(failurefp.Normalize(reason)))
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
