package audit

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var closureCycleRef = regexp.MustCompile(`cycle[- ]?(\d+)`)

func closureLineCycleRefs(line string) []int {
	var out []int
	for _, m := range closureCycleRef.FindAllStringSubmatch(stripPathTokens(strings.ToLower(line)), -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			out = append(out, n)
		}
	}
	return out
}

var (
	closureClaimRE       = regexp.MustCompile(`\bverified[ -]closed\b`)
	closureClosedTokenRE = regexp.MustCompile(`(?:^|[^-\w])closed\b`)
	closureNegationRE    = regexp.MustCompile(`\b(?:not|never|isn['’]?t|wasn['’]?t|aren['’]?t)\s+(?:\w+\s+){0,2}closed\b`)
	closureOpenAssertRE  = regexp.MustCompile(`\b(?:still|remains?|left|stays?)\s+open\b|\bre-?opened\b`)
)

var closureCitationArtifacts = []string{defectDispositionFile, defectLedgerFile}

func closureClaimOffenders(text string) []string {
	var offenders []string
	for _, line := range strings.Split(text, "\n") {
		lower := stripQuotedSpans(strings.ToLower(line))
		strong := closureClaimRE.MatchString(lower)
		weak := closureClosedTokenRE.MatchString(lower) && closureCycleRef.MatchString(stripPathTokens(lower)) &&
			!closureNegationRE.MatchString(lower) && !closureOpenAssertRE.MatchString(lower)
		if !strong && !weak {
			continue
		}
		cited := false
		for _, artifact := range closureCitationArtifacts {
			if strings.Contains(lower, artifact) {
				cited = true
				break
			}
		}
		if !cited {
			offenders = append(offenders, strings.TrimSpace(line))
		}
	}
	return offenders
}

func stripQuotedSpans(line string) string {
	r := []rune(line)
	var out []rune
	for i := 0; i < len(r); i++ {
		if !isQuoteDelim(r, i) {
			out = append(out, r[i])
			continue
		}
		close := -1
		for j := i + 1; j < len(r); j++ {
			if r[j] == r[i] && isQuoteDelim(r, j) {
				close = j
				break
			}
		}
		if close < 0 {
			out = append(out, r[i])
			continue
		}
		i = close
	}
	return string(out)
}

func stripPathTokens(line string) string {
	fields := strings.Fields(line)
	kept := fields[:0]
	for _, f := range fields {
		if !strings.Contains(f, "/") {
			kept = append(kept, f)
		}
	}
	return strings.Join(kept, " ")
}

func isQuoteDelim(r []rune, i int) bool {
	switch r[i] {
	case '"':
		return true
	case '\'':
		return i == 0 || !isLetterRune(r[i-1]) || i+1 >= len(r) || !isLetterRune(r[i+1])
	}
	return false
}

func isLetterRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func closureClaimDiagnostics(text string) []core.Diagnostic {
	offenders := closureClaimOffenders(text)
	if len(offenders) == 0 {
		return nil
	}
	diags := make([]core.Diagnostic, 0, len(offenders))
	for _, o := range offenders {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message: "closure claim without a citation: " + quoteClaim(o) +
				" — a report may not assert a prior cycle's defect is closed without naming the per-defect record on the same line (" +
				defectDispositionFile + " or " + defectLedgerFile + "). Assertion is what laundered the 1255 CRITICAL through four cycles.",
		})
	}
	return diags
}

func quoteClaim(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	return "\"" + truncateRunes(s, 200) + "\""
}
