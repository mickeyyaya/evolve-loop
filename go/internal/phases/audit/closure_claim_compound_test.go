package audit

import (
	"strings"
	"testing"
)

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
		"the retirement semantics are fail-closed by construction, root-causing a cycle-767 blindness along the way",
		"the connection is half-closed after cycle-1200's shutdown ordering fix",
		"coverage evidence: .evolve/runs/cycle-1493/coverage-gate-report.md:32 shows the floor is closed to overrides",
	}
	for _, line := range benign {
		if got := closureClaimOffenders(line + "\n"); len(got) != 0 {
			t.Errorf("false positive on benign line %q -> %v", line, got)
		}
	}
}

func TestClosureClaimOffenders_CompoundFixKeepsRealCatches(t *testing.T) {
	t.Parallel()
	offending := []string{
		"the cycle-1424 defect is closed",
		"cycle-1272: closed during this lane's build",
		"the cycle-1255 CRITICAL is verified closed",
		"the cycle-1424 defect is verified-closed",
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
