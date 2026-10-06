package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const laneDeferredTiming = `[{"phase":"start","verdict":"SKIPPED","abort_reason":"lane-worktree-deferred: git fetch origin main: rc=1: error: cannot lock ref 'refs/remotes/origin/main'"}]`

func writeTimingFixture(t *testing.T, evolveDir string, cycle int, timing string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectBatchLaneDeferrals_CountsOnlyWorktreeDeferralsInTheBatch(t *testing.T) {
	evolveDir := t.TempDir()
	writeTimingFixture(t, evolveDir, 9, laneDeferredTiming)
	writeTimingFixture(t, evolveDir, 10, laneDeferredTiming)
	writeTimingFixture(t, evolveDir, 11, `[{"phase":"build","verdict":"FAIL","abort_reason":"all-families-exhausted: drained"}]`)
	writeTimingFixture(t, evolveDir, 12, `[{"phase":"ship","verdict":"PASS"}]`)

	got := CollectBatchLaneDeferrals(evolveDir, 10)

	want := []LaneDeferralRecord{{Cycle: 10, Cause: "git fetch origin main: rc=1: error: cannot lock ref 'refs/remotes/origin/main'"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CollectBatchLaneDeferrals = %+v, want %+v: a quota DEFERRED, a ship and a pre-batch cycle never count", got, want)
	}
	if got := CollectBatchLaneDeferrals(filepath.Join(evolveDir, "absent"), 0); got != nil {
		t.Errorf("no runs dir yields nothing, got %+v", got)
	}
}

func TestEvaluateLaneDeferrals_HaltsOnlyOnAnUnbrokenRun(t *testing.T) {
	d := func(cycle int) LaneDeferralRecord {
		return LaneDeferralRecord{Cycle: cycle, Cause: "git fetch origin main: cannot lock ref 'refs/remotes/origin/main'"}
	}
	v := EvaluateLaneDeferrals([]LaneDeferralRecord{d(12), d(10), d(11)}, 3)
	if !v.Halt || v.Rule != "lane-deferrals" || v.Count != 3 || !strings.Contains(v.Reason, "cannot lock ref") || !strings.Contains(v.Reason, "cycle 12") {
		t.Errorf("three consecutive deferrals must halt naming the rule, the count, the last cycle and its cause: %+v", v)
	}
	if v := EvaluateLaneDeferrals([]LaneDeferralRecord{d(10), d(11), d(13)}, 3); v.Halt {
		t.Errorf("a cycle between deferrals that did not defer breaks the run: %+v", v)
	}
	if v := EvaluateLaneDeferrals([]LaneDeferralRecord{d(10), d(11), d(12)}, 0); v.Halt {
		t.Errorf("a zero ceiling disables the rule: %+v", v)
	}
}

func TestConsecutiveFailures_LaneDeferredCyclesAreTransparent(t *testing.T) {
	failed := []FailureDigest{
		{Cycle: 11, Fingerprint: "scout|infra|aaa", PreClass: "infra"},
		{Cycle: 13, Fingerprint: "build|verdict-fail|bbb", PreClass: "verdict-fail"},
		{Cycle: 15, Fingerprint: "audit|verdict-fail|ccc", PreClass: "verdict-fail"},
	}
	cfg := BlockerBreakerConfig{ConsecutiveFailuresCeiling: 3, LaneDeferredCycles: LaneDeferredCycleSet([]LaneDeferralRecord{{Cycle: 12}, {Cycle: 14}})}
	if v := EvaluateBlockerBreaker(failed, cfg); !v.Halt || v.Rule != "consecutive-failures" || v.Count != 3 {
		t.Errorf("F D F D F must halt as consecutive-failures: a deferral neither counts nor breaks the run, got %+v", v)
	}
	cfg.LaneDeferredCycles = map[int]bool{12: true}
	if v := EvaluateBlockerBreaker(failed, cfg); v.Halt {
		t.Errorf("F D F S F: a cycle that neither failed nor deferred (a ship) still breaks the run, got %+v", v)
	}
}
