package treediff

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeGit returns a GitDirtyFn that yields scripted snapshots: first call
// returns `before`, second returns `after`. Set err for a failure case.
func fakeGit(before, after []string, err error) GitDirtyFn {
	calls := 0
	return func(ctx context.Context, repoRoot string) ([]string, error) {
		if err != nil {
			return nil, err
		}
		calls++
		if calls == 1 {
			return before, nil
		}
		return after, nil
	}
}

func TestGuard_Snapshot_NoSeam_NoOp(t *testing.T) {
	// A guard with no seam (nil) must not panic and must yield nil paths —
	// callers degrade to "skip the post-phase check".
	g := New(nil)
	got, err := g.Snapshot(context.Background(), "/repo")
	if err != nil || got != nil {
		t.Errorf("nil seam: got (%v, %v), want (nil, nil)", got, err)
	}
}

func TestGuard_CleanTree_NoLeak(t *testing.T) {
	// Common case: clean tree before, clean tree after → no leak.
	g := New(fakeGit(nil, nil, nil))
	before, _ := g.Snapshot(context.Background(), "/repo")
	res := g.Check(context.Background(), "/repo", before)
	if !res.OK() {
		t.Errorf("clean→clean must be OK; got %+v", res)
	}
	if res.SnapshotMissed {
		t.Error("clean snapshot must not flag SnapshotMissed")
	}
}

func TestGuard_NewDirtyPath_FlagsLeak(t *testing.T) {
	// A path that wasn't dirty before but is dirty after → leak. This is the
	// cycle-119 cross-CLI bypass signature: Gemini scout wrote to main during
	// a read-only phase.
	before := []string{}
	after := []string{"docs/leaked.md"}
	g := New(fakeGit(before, after, nil))
	beforeSnap, _ := g.Snapshot(context.Background(), "/repo")
	res := g.Check(context.Background(), "/repo", beforeSnap)
	if res.OK() {
		t.Fatal("new dirty path must register as a leak")
	}
	if !equalLeaks(res.Leaked, []string{"docs/leaked.md"}) {
		t.Errorf("leaked=%v, want [docs/leaked.md]", res.Leaked)
	}
	// Error message must name the phase + worktree + leaked paths.
	msg := res.Error("build", "/wt/cycle-5").Error()
	for _, want := range []string{"build", "/wt/cycle-5", "docs/leaked.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error msg missing %q: %s", want, msg)
		}
	}
}

func TestGuard_PreviouslyDirty_NotALeak(t *testing.T) {
	// A path that was ALREADY dirty before the phase ran is NOT a leak — the
	// operator had uncommitted changes pre-cycle; the guard only flags NEW
	// dirtiness caused BY the phase.
	before := []string{"docs/operator-wip.md"}
	after := []string{"docs/operator-wip.md"}
	g := New(fakeGit(before, after, nil))
	beforeSnap, _ := g.Snapshot(context.Background(), "/repo")
	res := g.Check(context.Background(), "/repo", beforeSnap)
	if !res.OK() {
		t.Errorf("pre-existing dirt must not flag a leak; got %+v", res)
	}
}

func TestGuard_PostPhaseGitFails_DegradesNotAborts(t *testing.T) {
	// The post-phase git call can fail (transient lock, etc.). The guard
	// must NOT report leaks (SnapshotMissed=true) — it's belt-and-suspenders
	// to the OS sandbox, so a probe error must degrade silently.
	calls := 0
	fn := func(ctx context.Context, repoRoot string) ([]string, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return nil, errors.New("git locked")
	}
	g := New(fn)
	beforeSnap, _ := g.Snapshot(context.Background(), "/repo")
	res := g.Check(context.Background(), "/repo", beforeSnap)
	if !res.SnapshotMissed {
		t.Errorf("post-phase failure must set SnapshotMissed; got %+v", res)
	}
	if res.OK() {
		t.Error("SnapshotMissed result must not also report OK()")
	}
}

func TestCheckResult_Error_NoLeak_ReturnsNil(t *testing.T) {
	// A clean result (no leaks, snapshot worked) must produce no error — the
	// orchestrator uses a nil return to mean "phase respected its worktree".
	r := CheckResult{}
	if err := r.Error("build", "/wt/cycle-5"); err != nil {
		t.Errorf("clean result must yield nil error; got %v", err)
	}
}

func TestCheckResult_Error_SnapshotMissed_DegradeMessage(t *testing.T) {
	// When the pre-phase snapshot failed, Error names the phase and states
	// that leak detection was skipped — NOT a leak abort message. This is the
	// degrade-not-abort branch the orchestrator must distinguish.
	r := CheckResult{SnapshotMissed: true}
	err := r.Error("scout", "/wt/cycle-7")
	if err == nil {
		t.Fatal("SnapshotMissed must produce a (degrade) error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "scout") {
		t.Errorf("degrade message must name the phase; got %q", msg)
	}
	if !strings.Contains(msg, "snapshot failed") || !strings.Contains(msg, "skipped") {
		t.Errorf("degrade message must explain snapshot-failed/skipped; got %q", msg)
	}
	if strings.Contains(msg, "leaked paths") {
		t.Errorf("snapshot-missed message must NOT claim a leak; got %q", msg)
	}
}

func TestGuard_Check_NoSeam_OKByDefault(t *testing.T) {
	// A guard with no git seam is disabled: Check returns a zero CheckResult
	// (OK, no leaks, snapshot not missed) so a disabled guard never blocks.
	g := New(nil)
	res := g.Check(context.Background(), "/repo", []string{"docs/x.md"})
	if !res.OK() {
		t.Errorf("disabled guard must be OK; got %+v", res)
	}
	if res.SnapshotMissed || res.Leaked != nil {
		t.Errorf("disabled guard must yield zero result; got %+v", res)
	}
}

func equalLeaks(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
