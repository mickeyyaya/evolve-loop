package fixtures_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestFakeBridge_ArtifactWriteFailureIsReported(t *testing.T) {
	t.Parallel()
	scriptedErr := errors.New("scripted bridge failure")
	for _, tc := range []struct {
		name         string
		parentIsFile bool
		err          error
	}{
		{"parent_is_file", true, nil},
		{"target_is_directory", false, nil},
		{"parent_is_file_with_scripted_error", true, scriptedErr},
		{"target_is_directory_with_scripted_error", false, scriptedErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "blocked")
			if tc.parentIsFile {
				fixtures.MustWrite(t, path, "keep")
				path = filepath.Join(path, "report.md")
			} else {
				fixtures.RequireNoErr(t, os.Mkdir(path, 0o755), "mkdir target")
			}
			want := core.BridgeResponse{Stdout: "scripted", ExitCode: 3}
			fb := &fixtures.FakeBridge{Resp: want, Err: tc.err, WriteArtifact: "undelivered"}

			got, err := fb.Launch(context.Background(), core.BridgeRequest{ArtifactPath: path})

			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("artifact materialization error = %v, want filesystem error", err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Errorf("artifact materialization error = %v, want scripted error %v too", err, tc.err)
			}
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(fb.Resp, want) {
				t.Errorf("failed delivery changed response: returned %+v, stored %+v, want %+v", got, fb.Resp, want)
			}
		})
	}
}

func TestFakeBridge_ScriptedResponsePreserved(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"empty_artifact", "empty_path", "artifact_with_error"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.md")
			fixtures.MustWrite(t, path, "existing")
			scriptedErr := errors.New("scripted bridge failure")
			want := core.BridgeResponse{Stdout: "scripted", ExitCode: 3}
			fb := &fixtures.FakeBridge{Resp: want, Err: scriptedErr, WriteArtifact: "# exact\n\n"}
			req := core.BridgeRequest{ArtifactPath: path}
			switch mode {
			case "empty_artifact":
				fb.WriteArtifact = ""
			case "empty_path":
				req.ArtifactPath = ""
			case "artifact_with_error":
				want.Stdout = fb.WriteArtifact
			}

			got, err := fb.Launch(context.Background(), req)

			if !errors.Is(err, scriptedErr) || !reflect.DeepEqual(got, want) {
				t.Fatalf("Launch = (%+v, %v), want (%+v, %v)", got, err, want, scriptedErr)
			}
			wantFile := "existing"
			if mode == "artifact_with_error" {
				wantFile = fb.WriteArtifact
			}
			if gotFile := fixtures.MustRead(t, path); gotFile != wantFile {
				t.Errorf("artifact bytes = %q, want %q", gotFile, wantFile)
			}
		})
	}
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
	for call, offset := range []time.Duration{0, 200 * time.Millisecond, 400 * time.Millisecond, 600 * time.Millisecond} {
		if got, want := clock(), start.Add(offset); !got.Equal(want) {
			t.Fatalf("call %d = %v, want %v", call+1, got, want)
		}
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
