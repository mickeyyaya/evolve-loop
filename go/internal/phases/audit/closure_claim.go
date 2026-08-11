package audit

import (
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// closure_claim.go — the closure-citation gate (cycle-1285 Task 2; inbox item
// `continuation-defect-ledger` clause (3), batch-integrity-review-2026-08-04.md:123).
//
// The 1255 → 1268 → 1270 → 1272 chain closed a named CRITICAL by ASSERTION: a
// bookkeeping line reading "verified closed" was the entire proof, and nothing
// required it to point at a record a reader could check. defect_ledger.go now
// MINTS that record (defect-ledger.json / defect-dispositions.json); this gate
// makes citing it mandatory whenever a report claims a prior cycle's defect is
// closed. Without it the ledger is a filing cabinet nobody is obliged to open.
//
// LINE-scoped, deliberately. A whole-document reading — "the file mentions
// defect-dispositions.json somewhere, so every closure claim in it is cited" —
// is the loophole, not the feature: one incidental mention would vouch for
// twenty unevidenced claims.

// closureCycleRef matches a cycle reference in prose ("cycle-1272", "cycle 1255").
var closureCycleRef = regexp.MustCompile(`cycle[- ]?\d+`)

// The closure-claim token matchers (cycle-1431 lesson — see
// closureClaimOffenders and the stripQuotedSpans design record): word-bounded
// so "disclosed"/"foreclosed" never match; the negation/openness guards apply
// to the WEAK rung only. The negation vocabulary is deliberately small
// ("hasn't/won't/cannot be closed" still flag) — grow it from firings, not
// speculation.
var (
	closureClaimRE       = regexp.MustCompile(`\bverified closed\b`)
	closureClosedTokenRE = regexp.MustCompile(`\bclosed\b`)
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
		// Two rungs (cycle-1431 lesson — see the matcher var block): the
		// STRONG rung ("verified closed") is never guard-suppressed — an
		// appended "…still open" clause must not become a one-token bypass of
		// the citation demand; only the WEAK rung (bare "closed" + cycle-ref)
		// accepts the negation/openness guards, whose whole job is the
		// disclosed/"still open" false-RED class.
		strong := closureClaimRE.MatchString(lower)
		weak := closureClosedTokenRE.MatchString(lower) && closureCycleRef.MatchString(lower) &&
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

// stripQuotedSpans removes text between matched quotation marks so the gate
// matches an ASSERTION of closure rather than the mere presence of the phrase
// (cycle-1285 F5).
//
// The canonical inherited defect text in this repo literally contains the words
// "verified closed" — batch-integrity-review-2026-08-04.md reports the 1255-D1
// CRITICAL as having been «narrowed to 'verified closed'». Substring matching
// therefore FAILED the auditor who correctly reported that defect as still
// open, which is worse than useless: the cheapest way out is to append the
// literal token "defect-dispositions.json" to the line, which satisfies the
// gate and adds no evidence at all. A gate whose remedy is a one-token
// appeasement becomes noise and then gets deleted.
//
// Quoting is the signal because it is what the honest report actually does:
// quoting someone else's closure claim is reporting, asserting one unquoted is
// claiming. Explicit negation markers ("still open", "not closed") were
// originally REJECTED as a second signal — they would hand the gate a bypass
// strictly cheaper than the citation it demands, since appending "not closed"
// is one token and evidences nothing. Cycle-1431 (with prior firings
// 1339/1371/1428) revised that posture for the WEAK rung only: four P0
// false-RED batch halts on honest refutations outweigh a one-rung leak that
// the per-id dispositions gate still backstops, so bare-"closed"+cycle-ref
// lines accept the negation/openness guards. The STRONG rung ("verified
// closed") keeps the original rejection in full — no guard suppresses it —
// so the one-token bypass remains closed where the claim is unambiguous.
// Quoting cannot be used the same way: a claim wrapped in quotes reads as
// someone else's.
//
// Backticks are NOT delimiters. In markdown a code span is how a real citation
// is written (`defect-dispositions.json`), so stripping them would erase the
// evidence and manufacture offenders.
//
// An unmatched delimiter strips nothing: a line with one apostrophe must not
// swallow its own tail. A `'` is a delimiter only when it is not word-internal,
// which keeps ordinary possessives ("the gate's record") out of the pairing.
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
