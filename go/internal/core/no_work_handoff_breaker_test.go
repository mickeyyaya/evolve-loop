package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeNoWorkFixture(t *testing.T, evolveDir string, cycle int, timing string) {
	t.Helper()
	d := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	noWorkTiming  = `[{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"}]`
	shippedTiming = `[{"phase":"ship","verdict":"PASS"}]`
	failedTiming  = `[{"phase":"build","verdict":"FAIL","abort_reason":"build-contract-gate-block"}]`
)

func TestCollectBatchNoWorkHandoffs_CountsOnlyThisBatchsPlannedNoWorkLanes(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	for cycle, timing := range map[int]string{9: noWorkTiming, 10: noWorkTiming, 11: shippedTiming, 12: noWorkTiming, 13: failedTiming} {
		writeNoWorkFixture(t, evolveDir, cycle, timing)
	}

	got := CollectBatchNoWorkHandoffs(evolveDir, 10)

	if len(got) != 2 || got[0] != 10 || got[1] != 12 {
		t.Fatalf("CollectBatchNoWorkHandoffs = %v, want [10 12]: the batch's no-work lanes, sorted, not cycle 9 of an earlier batch nor a ship or a FAIL", got)
	}
	if got := CollectBatchNoWorkHandoffs(filepath.Join(evolveDir, "absent"), 1); len(got) != 0 {
		t.Fatalf("a missing runs dir counts nothing: %v", got)
	}
}

func TestEvaluateNoWorkHandoffs_HaltsAtTheCeilingEvenWhenOtherLanesShip(t *testing.T) {
	t.Parallel()

	if v := EvaluateNoWorkHandoffs([]int{11, 14, 17}, 4); v.Halt {
		t.Fatalf("three hand-offs under a ceiling of four halted: %+v", v)
	}
	v := EvaluateNoWorkHandoffs([]int{11, 14, 17, 20}, 4)
	if !v.Halt || v.Rule != noWorkHandoffsRule || v.Count != 4 {
		t.Fatalf("four hand-offs at a ceiling of four must halt with the rule and the count: %+v", v)
	}
	for _, want := range []string{"4 planned no-work lanes", "cycles 11, 14, 17, 20", "ceiling 4"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("the reason lacks %q: %s", want, v.Reason)
		}
	}
	if v := EvaluateNoWorkHandoffs([]int{1, 2, 3, 4, 5}, 0); v.Halt {
		t.Fatalf("a non-positive ceiling disables the rule: %+v", v)
	}
}
