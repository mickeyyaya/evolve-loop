package audit

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// closureCycleRef matches a cycle reference in prose ("cycle-1272", "cycle
// 1255"), with the number captured, so the weak rung's reference check and
// closureLineCycleRefs share one pattern and cannot drift apart.
var closureCycleRef = regexp.MustCompile(`cycle[- ]?(\d+)`)

// closureLineCycleRefs returns the cycle numbers a line references in PROSE
// (path tokens dropped, same rule as the weak rung's ref check).
func closureLineCycleRefs(line string) []int {
	var out []int
	for _, m := range closureCycleRef.FindAllStringSubmatch(stripPathTokens(strings.ToLower(line)), -1) {
		if n, err := strconv.Atoi(m[1]); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// The token matchers are word-bounded so "disclosed"/"foreclosed" never
// match; the negation/openness guards apply to the weak rung only. The
// negation vocabulary stays deliberately small — grow it from real firings,
// not speculation.
var (
	// The hyphen-compound carve-out ([ -]) must not turn "verified-closed" into
	// a miss on either rung; the strong rung accepts the hyphenated spelling
	// directly, and "verified fail-closed" still cannot match, because the
	// compound's own hyphen sits between "fail" and "closed".
	closureClaimRE = regexp.MustCompile(`\bverified[ -]closed\b`)
	// `(?:^|[^-\w])` instead of `\b`: a hyphen is itself a word boundary, so a
	// plain \bclosed\b matches inside "fail-closed", a state adjective, not a
	// closure claim. Letter-prefixed compounds ("disclosed") stay excluded
	// because a letter is rejected by [^-\w] just as it was by \b.
	closureClosedTokenRE = regexp.MustCompile(`(?:^|[^-\w])closed\b`)
	closureNegationRE    = regexp.MustCompile(`\b(?:not|never|isn['’]?t|wasn['’]?t|aren['’]?t)\s+(?:\w+\s+){0,2}closed\b`)
	closureOpenAssertRE  = regexp.MustCompile(`\b(?:still|remains?|left|stays?)\s+open\b|\bre-?opened\b`)
)

// closureCitationArtifacts are the per-defect disposition records. A closure
// claim must name one of them ON THE SAME LINE.
var closureCitationArtifacts = []string{defectDispositionFile, defectLedgerFile}

// closureClaimOffenders returns every line of text that claims a prior cycle's
// defect is closed without citing the per-defect disposition record on that
// same line. Offenders are returned verbatim (trimmed) so the diagnostic can
// QUOTE them — a bare count is unactionable, because the operator cannot tell
// which of forty bookkeeping lines to fix.
//
// A claim is either the canonical phrase "verified closed", or the weaker
// "closed" when the same line also references a cycle — "the file handle is
// closed in the deferred cleanup" is prose, not a closure claim, and a gate
// that trips on it would be turned off within a cycle.
func closureClaimOffenders(text string) []string {
	var offenders []string
	for _, line := range strings.Split(text, "\n") {
		lower := stripQuotedSpans(strings.ToLower(line))
		// The strong rung ("verified closed") is never guard-suppressed, so an
		// appended "…still open" clause cannot become a one-token bypass of the
		// citation demand; only the weak rung (bare "closed" + cycle-ref)
		// accepts the negation/openness guards.
		strong := closureClaimRE.MatchString(lower)
		// The weak rung's cycle reference must come from prose, not from a
		// path: a citation locator like `.evolve/runs/cycle-N/…` is not itself
		// a prose claim about that cycle. Tokens containing '/' are dropped for
		// this one check; the closed-token, negation, and citation checks keep
		// the full line.
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

// stripQuotedSpans removes text between matched quotation marks, so the gate
// matches an assertion of closure rather than the mere presence of the
// phrase: the repo's own canonical defect text quotes "verified closed" while
// correctly reporting the defect as still open. Backticks are not delimiters,
// since a markdown code span is how a real citation is written. An unmatched
// delimiter strips nothing, and a `'` is a delimiter only when it is not
// word-internal, so an ordinary possessive is left alone.
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
			out = append(out, r[i]) // unmatched: not a quotation, keep it
			continue
		}
		i = close // drop the delimiters and everything between them
	}
	return string(out)
}

// stripPathTokens drops whitespace-delimited tokens containing '/': file
// paths and locators. Used only for the weak rung's cycle-reference check, so
// a cycle number inside a citation path is not read as a prose claim about
// that cycle. A markdown-link ref and a dual-ref token also strip; both are
// weak-rung shapes an author could already evade by omitting the ref
// outright, and the strong rung plus the citation demand still stand.
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

// isQuoteDelim reports whether r[i] opens or closes a quotation. Double quotes
// always do; an apostrophe does not when it sits inside a word.
func isQuoteDelim(r []rune, i int) bool {
	switch r[i] {
	case '"':
		return true
	case '\'':
		return i == 0 || !isLetterRune(r[i-1]) || i+1 >= len(r) || !isLetterRune(r[i+1])
	}
	return false
}

// isLetterRune reports whether r is an ASCII letter — the only alphabet a
// word-internal apostrophe needs to be recognised in for this heuristic.
func isLetterRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// closureClaimDiagnostics renders one error diagnostic per uncited claim, each
// naming the remedy artifact and quoting the offending line.
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

// quoteClaim renders an offending line bounded and single-line, so an injected
// newline cannot forge extra diagnostic lines in the dossier.
func quoteClaim(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	return "\"" + truncateRunes(s, 200) + "\""
}
