package ship

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
)

// TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption (cycle-1691
// audit H2) pins why the verdict-string consumption gate is sufficient on the
// cycle path: the WARN + red_count:0 quadrant from the 2026-08-16 inbox item
// cannot pass the EGPS reader that verifyClass → verifyPredicateReceipt runs
// before atomicShip, so it never reaches consumeCommittedItems. The PASS twin
// of the same record is the positive control — the refusal is about the
// verdict, not a malformed fixture. If ReadVerdict ever tolerates WARN again,
// this goes red and the consumption gate must be revisited with it.
func TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption(t *testing.T) {
	pass := predicateVerdictFixture(1691, 3, 0, 1)
	if _, err := checkEGPSGate(writeVerdictDoc(t, pass), &RunResult{}); err != nil {
		t.Fatalf("positive control: a well-formed PASS verdict must clear the EGPS gate: %v", err)
	}

	warn := pass
	warn.Verdict = "WARN"
	if _, err := checkEGPSGate(writeVerdictDoc(t, warn), &RunResult{}); err == nil {
		t.Fatal("a WARN verdict with red_count 0 must be refused before a cycle ship can consume its inbox items")
	}

	unfinished := pass
	unfinished.Verdict = "FAIL"
	unfinished.ShipEligible = false
	if _, err := checkEGPSGate(writeVerdictDoc(t, unfinished), &RunResult{}); err == nil {
		t.Fatal("a FAIL / ship_eligible:false verdict with red_count 0 must be refused on the cycle path")
	}
}

func writeVerdictDoc(t *testing.T, v acssuite.Verdict) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}
	path := filepath.Join(t.TempDir(), "acs-verdict.json")
	mustWrite(t, path, string(raw))
	return path
}
