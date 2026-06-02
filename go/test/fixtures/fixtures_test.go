package fixtures_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestFakeStorage_RoundTripsStateAndCycleState(t *testing.T) {
	t.Parallel()
	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}
	ctx := context.Background()

	got, err := st.ReadState(ctx)
	fixtures.RequireNoErr(t, err, "ReadState")
	if got.LastCycleNumber != 7 {
		t.Fatalf("got LastCycleNumber=%d, want 7", got.LastCycleNumber)
	}

	if err := st.WriteCycleState(ctx, core.CycleState{CycleID: 7, Phase: "build"}); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}
	if len(st.CycleStateLog) != 1 || st.CycleStateLog[0].Phase != "build" {
		t.Fatalf("CycleStateLog not recorded: %+v", st.CycleStateLog)
	}
}

func TestFakeStorage_LockIsExclusiveAndCounts(t *testing.T) {
	t.Parallel()
	st := &fixtures.FakeStorage{}
	ctx := context.Background()

	release, err := st.AcquireLock(ctx)
	fixtures.RequireNoErr(t, err, "first AcquireLock")
	if _, err := st.AcquireLock(ctx); !errors.Is(err, core.ErrLockHeld) {
		t.Fatalf("second AcquireLock: got %v, want ErrLockHeld", err)
	}
	fixtures.RequireNoErr(t, release(), "release")
	if _, err := st.AcquireLock(ctx); err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if st.LockCount != 2 {
		t.Fatalf("LockCount=%d, want 2", st.LockCount)
	}
}

func TestFakeStorage_LockReleaseFn_OverridesRelease(t *testing.T) {
	t.Parallel()
	relErr := errors.New("release boom")
	st := &fixtures.FakeStorage{LockReleaseFn: func() error { return relErr }}
	release, err := st.AcquireLock(context.Background())
	fixtures.RequireNoErr(t, err, "AcquireLock")
	if got := release(); !errors.Is(got, relErr) {
		t.Fatalf("release() = %v, want the scripted error", got)
	}
}

func TestFakeStorage_WriteCycleStateFailAt_FailsNthCall(t *testing.T) {
	t.Parallel()
	st := &fixtures.FakeStorage{WriteCycleStateFailAt: 2}
	ctx := context.Background()
	fixtures.RequireNoErr(t, st.WriteCycleState(ctx, core.CycleState{}), "1st write")
	fixtures.RequireErr(t, st.WriteCycleState(ctx, core.CycleState{}), "2nd write should fail")
}

func TestFakeLedger_IterReplaysAppendOrder(t *testing.T) {
	t.Parallel()
	led := &fixtures.FakeLedger{}
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		fixtures.RequireNoErr(t, led.Append(ctx, fixtures.NewLedgerEntry(fixtures.WithCycle(i))), "append")
	}
	it, err := led.Iter(ctx)
	fixtures.RequireNoErr(t, err, "Iter")
	defer it.Close()
	var cycles []int
	for {
		e, ok, err := it.Next()
		fixtures.RequireNoErr(t, err, "Next")
		if !ok {
			break
		}
		cycles = append(cycles, e.Cycle)
	}
	if len(cycles) != 3 || cycles[0] != 1 || cycles[2] != 3 {
		t.Fatalf("iter order = %v, want [1 2 3]", cycles)
	}
}

func TestFakeRunner_TransientFailureThenSuccess(t *testing.T) {
	t.Parallel()
	r := &fixtures.FakeRunner{PhaseName: "build", FailErr: errors.New("boom"), FailUntil: 1}
	ctx := context.Background()
	if _, err := r.Run(ctx, core.PhaseRequest{}); err == nil {
		t.Fatal("first call should fail")
	}
	resp, err := r.Run(ctx, core.PhaseRequest{Workspace: "/ws"})
	fixtures.RequireNoErr(t, err, "second call")
	if resp.Verdict != core.VerdictPASS || resp.ArtifactsDir != "/ws" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
	if r.Calls != 2 {
		t.Fatalf("Calls=%d, want 2", r.Calls)
	}
}

func TestBuildRunners_CoversEveryPhaseWithOverrides(t *testing.T) {
	t.Parallel()
	runners := fixtures.BuildRunners(map[core.Phase]string{core.PhaseAudit: core.VerdictFAIL})
	if len(runners) != 9 {
		t.Fatalf("got %d runners, want 9", len(runners))
	}
	resp, _ := runners[core.PhaseAudit].Run(context.Background(), core.PhaseRequest{})
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("audit verdict = %q, want FAIL", resp.Verdict)
	}
	resp, _ = runners[core.PhaseScout].Run(context.Background(), core.PhaseRequest{})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("scout verdict = %q, want default PASS", resp.Verdict)
	}
}

func TestFakeBridge_MaterializesArtifact(t *testing.T) {
	t.Parallel()
	art := filepath.Join(t.TempDir(), "out", "report.md")
	fb := &fixtures.FakeBridge{WriteArtifact: "# hello\n"}
	resp, err := fb.Launch(context.Background(), core.BridgeRequest{ArtifactPath: art})
	fixtures.RequireNoErr(t, err, "Launch")
	if resp.Stdout != "# hello\n" {
		t.Fatalf("stdout = %q", resp.Stdout)
	}
	fixtures.WantFileContains(t, art, "hello")
}

func TestWorkspaceBuilder_SeedsStateCycleStateAndFiles(t *testing.T) {
	t.Parallel()
	ws := fixtures.NewWorkspace(t).
		WithState(core.State{LastCycleNumber: 4}).
		WithCycleState(core.CycleState{CycleID: 4, Phase: "scout"}).
		WithFiles(map[string]string{"docs/x.md": "body"}).
		WithCycleFiles(4, map[string]string{"scout-report.md": "found"}).
		Build()

	if !fixtures.FilePresent(filepath.Join(ws.EvolveDir, "state.json")) {
		t.Fatal("state.json missing")
	}
	fixtures.WantFileContains(t, filepath.Join(ws.EvolveDir, "cycle-state.json"), "scout")
	fixtures.WantFileContains(t, ws.Path("docs/x.md"), "body")
	fixtures.WantFileContains(t, filepath.Join(ws.CycleDir(4), "scout-report.md"), "found")
}

func TestFixedClock_AdvancesLinearly(t *testing.T) {
	t.Parallel()
	start := time.Unix(1_700_000_000, 0)
	clock := fixtures.FixedClock(start, 200*time.Millisecond)
	if !clock().Equal(start) {
		t.Fatal("first call should equal start")
	}
	if got := clock(); !got.Equal(start.Add(200 * time.Millisecond)) {
		t.Fatalf("second call = %v, want start+200ms", got)
	}
}

func TestFilePresent_PureBoolean(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if fixtures.FilePresent(filepath.Join(dir, "nope")) {
		t.Fatal("missing file reported present")
	}
	present := fixtures.MustWrite(t, filepath.Join(dir, "yes"), "x")
	if !fixtures.FilePresent(present) {
		t.Fatal("written file reported absent")
	}
}
