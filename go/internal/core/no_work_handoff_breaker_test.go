package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func writeHandoffFixture(t *testing.T, evolveDir string, cycle int, handedOff bool) {
	t.Helper()
	d := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if handedOff {
		if err := committedset.WriteHandoff(d, committedset.Handoff{Cycle: cycle, Routed: []string{"item-" + strconv.Itoa(cycle)}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCollectNoWorkHandoffs_ReadsTheLastWindowOfCyclesAboveTheAckMark(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	for cycle := 1; cycle <= 10; cycle++ {
		writeHandoffFixture(t, evolveDir, cycle, cycle%2 == 0)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "runs", "not-a-cycle"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := CollectNoWorkHandoffs(evolveDir, 0, 4); len(got) != 2 || got[0] != 8 || got[1] != 10 {
		t.Fatalf("window 4 over cycles 1..10 = %v, want [8 10]: only the last four cycles count, across batches", got)
	}
	if got := CollectNoWorkHandoffs(evolveDir, 8, 12); len(got) != 1 || got[0] != 10 {
		t.Fatalf("ack mark 8 = %v, want [10]: an acknowledged hand-off never counts again", got)
	}
	if got := CollectNoWorkHandoffs(filepath.Join(evolveDir, "absent"), 0, 12); len(got) != 0 {
		t.Fatalf("a missing runs dir counts nothing: %v", got)
	}
}

func TestEvaluateNoWorkHandoffs_HaltsAtTheCeilingWithAnAckableFingerprint(t *testing.T) {
	t.Parallel()

	if v := EvaluateNoWorkHandoffs([]int{11, 14, 17}, 4); v.Halt {
		t.Fatalf("three hand-offs under a ceiling of four halted: %+v", v)
	}
	v := EvaluateNoWorkHandoffs([]int{11, 14, 17, 20}, 4)
	if !v.Halt || v.Rule != noWorkHandoffsRule || v.Count != 4 || v.Fingerprint != NoWorkHandoffFingerprint(20) {
		t.Fatalf("four hand-offs at a ceiling of four must halt with the rule, the count and the fingerprint of the last cycle: %+v", v)
	}
	for _, want := range []string{"4 no-work lanes", "cycles 11, 14, 17, 20", "ceiling 4"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("the reason lacks %q: %s", want, v.Reason)
		}
	}
	if fp, ok := ParseConsumptionFingerprint("Evidence: x (rule=no-work-handoffs fingerprint=" + v.Fingerprint + ")"); !ok || fp != v.Fingerprint {
		t.Fatalf("the halt's P0 notes must carry a fingerprint the consumption reconcile can ack: %q %v", fp, ok)
	}
	if v := EvaluateNoWorkHandoffs([]int{1, 2, 3, 4, 5}, 0); v.Halt {
		t.Fatalf("a non-positive ceiling disables the rule: %+v", v)
	}
}

func TestNoWorkHandoffsResolvedThrough_IsTheHighestAckedCycle(t *testing.T) {
	t.Parallel()

	acked := map[string]bool{NoWorkHandoffFingerprint(12): true, NoWorkHandoffFingerprint(30): true, "build|verdict-fail|abc": true, "no-work-handoffs|cycle|x": true, "99": true}
	if got := NoWorkHandoffsResolvedThrough(acked); got != 30 {
		t.Fatalf("resolved through %d, want 30", got)
	}
	if got := NoWorkHandoffsResolvedThrough(nil); got != 0 {
		t.Fatalf("no ack resolves nothing: %d", got)
	}
}
