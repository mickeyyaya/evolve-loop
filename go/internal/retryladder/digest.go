package retryladder

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	DefaultDigestTokens = policy.DefaultRetryLadderDigestTokens
	DigestBytesPerToken = 4
	digestCutMarker     = "\n\n[digest cut at the token budget]"
)

type Digest struct {
	Rung         Rung
	Class        Class
	Failures     []string
	Findings     string
	ChangedFiles []string
	History      []string
}

func (d Digest) Render(budgetTokens int) string {
	if budgetTokens <= 0 {
		budgetTokens = DefaultDigestTokens
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Retry digest (rung %s, class %s)\n\n", d.Rung, d.Class)
	b.WriteString("This try runs in a new session. It does not have the transcript of the last try. Use the facts below and fix the failed checks.\n")
	writeDigestPart(&b, "Failed checks (verbatim)", d.Failures)
	if d.Findings != "" {
		writeDigestPart(&b, "Audit findings", []string{d.Findings})
	}
	writeDigestList(&b, "Changed files", d.ChangedFiles)
	writeDigestList(&b, "Tries before this one", d.History)
	return cutToBytes(b.String(), budgetTokens*DigestBytesPerToken)
}

func writeDigestPart(b *strings.Builder, title string, blocks []string) {
	if len(blocks) == 0 {
		return
	}
	fmt.Fprintf(b, "\n### %s\n\n%s\n", title, strings.Join(blocks, "\n\n"))
}

func writeDigestList(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "\n### %s\n\n", title)
	for _, line := range lines {
		fmt.Fprintf(b, "- %s\n", strings.TrimSpace(line))
	}
}

func cutToBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	keep := max(0, limit-len(digestCutMarker))
	for keep > 0 && !utf8.RuneStart(s[keep]) {
		keep--
	}
	return s[:keep] + digestCutMarker
}
