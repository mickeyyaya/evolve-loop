package audit

// closure_claim_compound_test.go — RED contract for the cycle-1493 infra-systemic
// halt (batch-20260816c): two more weak-rung false-positive classes, both live-fired.
// (1) HYPHEN COMPOUNDS: `\bclosed\b` matches inside "fail-closed" — the hyphen is a
// word boundary, so the cycle-1431 fix's "disclosed/foreclosed never match" guarantee
// does not extend to hyphenated adjectives. (2) PATH-SHAPED CYCLE REFS: the weak rung's
// cycle-reference requirement was satisfied by the report's OWN evidence path
// (`.evolve/runs/cycle-1493/coverage-gate-report.md:32-36`) — a citation locator, not a
// prose claim about a prior cycle. Line 36 of the live audit-report asserted the
// inherited defect was "reproduced, not fixed" (the OPPOSITE of closure), carried
// "closed" only inside two "fail-closed" tokens and a cycle ref only inside its
// evidence path — and still force-FAILed a narrative-green audit. Cycle-1486's two
// closure flags ("fail-closed by construction" prose) were the same class.

import (
	"strings"
	"testing"
)

// The live cycle-1493 audit-report.md line 36, byte-shape preserved (trimmed of the
// table's trailing spaces): every signal on it is a false one.
const cycle1493Line36 = "| H3 | HIGH | Inherited defect `d8e3cdca…` is reproduced, not fixed: the coverage gate FAILs at 75.6% changed-line coverage against the 85% floor, with the new `internal/dispatchgate` package at 66.7% and 33 uncovered changed lines. The uncovered lines remain the fail-closed error branches this feature's safety argument rests on. Root cause: the fail-closed branches (`retire.go:144-161`, `dispatchgate.go:58-72`) are reachable only through store-write failure and are not exercised by any test. | `.evolve/runs/cycle-1493/coverage-gate-report.md:32-36` |"

func TestClosureClaimOffenders_Cycle1493LiveLineIsBenign(t *testing.T) {
	t.Parallel()
	if got := closureClaimOffenders(cycle1493Line36 + "\n"); len(got) != 0 {
		t.Errorf("cycle-1493 line 36 flagged as a closure claim: 'closed' appears only inside 'fail-closed', the only cycle ref is the report's own evidence path, and the line asserts the defect is reproduced, not fixed -> %v", got)
	}
}

func TestClosureClaimOffenders_HyphenCompoundsAndPathRefs(t *testing.T) {
	t.Parallel()
	benign := []string{
		// hyphen compounds: adjectives, not claims
		"the retirement semantics are fail-closed by construction, root-causing a cycle-767 blindness along the way",
		"the connection is half-closed after cycle-1200's shutdown ordering fix",
		// cycle ref only inside a path/locator token
		"coverage evidence: .evolve/runs/cycle-1493/coverage-gate-report.md:32 shows the floor is closed to overrides",
	}
	for _, line := range benign {
		if got := closureClaimOffenders(line + "\n"); len(got) != 0 {
			t.Errorf("false positive on benign line %q -> %v", line, got)
		}
	}
}

// The two new carve-outs must not weaken any real catch: prose cycle refs with a
// genuine bare "closed" still flag, the strong rung is untouched, and a citation
// on the same line still clears.
func TestClosureClaimOffenders_CompoundFixKeepsRealCatches(t *testing.T) {
	t.Parallel()
	offending := []string{
		"the cycle-1424 defect is closed",
		"cycle-1272: closed during this lane's build",
		"the cycle-1255 CRITICAL is verified closed", // strong rung
		// review HIGH-1: the hyphen carve-out must not make the one-character
		// mutation of the canonical phrase a both-rungs miss — strong rung
		// accepts the hyphenated spelling.
		"the cycle-1424 defect is verified-closed",
		// a path on the line does not launder a separate prose cycle ref + claim
		"cycle-1272 is closed; see .evolve/runs/cycle-1300/notes.md",
	}
	for _, line := range offending {
		if got := closureClaimOffenders(line + "\n"); len(got) != 1 {
			t.Errorf("real claim not caught: %q -> %v", line, got)
		}
	}
	cited := "the cycle-1424 defect is closed — defect-dispositions.json entry d8e3cdca"
	if got := closureClaimOffenders(cited + "\n"); len(got) != 0 {
		t.Errorf("cited claim must clear: %q -> %v", cited, got)
	}
	if !strings.Contains(cycle1493Line36, "fail-closed") {
		t.Fatal("fixture drifted: live line must contain the fail-closed compound")
	}
}

// Documented ACCEPTED MISSES of the path-token strip (review MEDIUM-3): these
// weak-rung shapes no longer flag. Pinned so a future tightening flips them
// deliberately, not accidentally — and because the evasion they open is
// cost-equivalent to omitting the cycle ref, which was always free. The
// strong rung and the citation demand still apply to such lines.
func TestClosureClaimOffenders_PathStripAcceptedMisses(t *testing.T) {
	t.Parallel()
	accepted := []string{
		"[cycle-1272](docs/runs/cycle-1272.md) is closed",
		"the cycle-1272/cycle-1273 defect pair is closed",
	}
	for _, line := range accepted {
		if got := closureClaimOffenders(line + "\n"); len(got) != 0 {
			t.Errorf("accepted-miss shape now flags — deliberate tightening? update this pin and the stripPathTokens doc together: %q -> %v", line, got)
		}
	}
}
