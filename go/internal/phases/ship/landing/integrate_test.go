package landing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const fastForwardProbe = "merge-base --is-ancestor " + testBranch + " " + testHead

func integration(fleet bool) Integration {
	return Integration{Branch: testBranch, CycleBranch: testCycleBr, Commit: testHead, Binary: "go/evolve", Fleet: fleet}
}

func resetFailureFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	f.set("checkout HEAD -- go/evolve", scripted{exit: 1})
	l.Integrate(context.Background(), integration(false))
	f.set("checkout HEAD -- go/evolve", scripted{})
}

func advanceFailureFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	f.set("merge --ff-only "+testHead, scripted{exit: 128})
	l.Integrate(context.Background(), integration(false))
	f.set("merge --ff-only "+testHead, scripted{})
}

func TestIntegrate_ResetThenFFMergeOfThePushedCommit_ArgvOrderAndStreams(t *testing.T) {
	f := newFakeGit()
	l, got := newLanding(f)
	sink, lines := logSink()
	req := integration(false)
	req.Log = sink

	l.Integrate(context.Background(), req)

	want := []string{"checkout HEAD -- go/evolve [discard]", "merge --ff-only " + testHead + " [streams]"}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("argv\n got %q\nwant %q", f.calls, want)
	}
	if strings.Join(*lines, "\n") != "[ship]   OK: ff-merged "+testCycleBr+" into "+testBranch {
		t.Errorf("log lines %q", *lines)
	}
	if len(*got) != 0 {
		t.Errorf("a green integrate emits nothing: %+v", *got)
	}
}

func TestIntegrate_ResetFailure_EmitsBinaryResetFailedAndProceeds(t *testing.T) {
	for name, call := range map[string]scripted{"rc=1": {exit: 1}, "spawn error": {err: errors.New("no git")}} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGit()
			f.on("checkout HEAD -- go/evolve", call)
			l, got := newLanding(f)

			l.Integrate(context.Background(), integration(false))

			if len(f.calls) != 2 || !strings.HasPrefix(f.calls[1], "merge --ff-only") {
				t.Errorf("the merge still runs after a failed reset: %v", f.calls)
			}
			rc, gitErr := "1", ""
			if call.err != nil {
				rc, gitErr = "0", "no git"
			}
			e := wantOneEvent(t, *got, CodeBinaryResetFailed, "Landing.Integrate",
				map[string]string{"step": "integrate", "path": "go/evolve", "git_rc": rc, "git_err": gitErr})
			if !strings.HasPrefix(e.Reason, "could not reset go/evolve to HEAD (exit="+rc+", err=") || !strings.HasSuffix(e.Reason, "); ff-merge may still fail if it is dirty") {
				t.Errorf("reason %q", e.Reason)
			}
			if f.stderr.Len() != 0 {
				t.Errorf("the leaf never writes the operator stderr itself: %q", f.stderr.String())
			}
		})
	}
}

func TestIntegrate_AFastForwardThatFailsAfterThePushWarnsAndLogsInsteadOfFailing(t *testing.T) {
	for name, call := range map[string]scripted{"rc=128": {exit: 128}, "spawn error": {err: errors.New("no git")}} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGit()
			f.on("merge --ff-only "+testHead, call)
			l, got := newLanding(f)
			sink, lines := logSink()
			req := integration(true)
			req.Log = sink

			l.Integrate(context.Background(), req)

			rc, gitErr := "128", ""
			if call.err != nil {
				rc, gitErr = "0", "no git"
			}
			e := wantOneEvent(t, *got, CodeAdvanceFailed, "Landing.Integrate",
				map[string]string{"step": "integrate", "branch": testBranch, "commit": testHead, "git_rc": rc, "git_err": gitErr})
			if !strings.Contains(e.Reason, "origin holds the commit") {
				t.Errorf("reason %q, want it to say the landing already happened on origin", e.Reason)
			}
			if len(*lines) != 1 || !strings.HasPrefix((*lines)[0], "[ship] WARN: the fast-forward of main to the pushed "+testHead) {
				t.Errorf("log lines %q, want one WARN and no OK line", *lines)
			}
		})
	}
}

func TestCheckFastForward_AnAncestorBranchPassesSilently(t *testing.T) {
	f := newFakeGit()
	l, got := newLanding(f)

	err := l.CheckFastForward(context.Background(), integration(true))

	if err != nil || len(*got) != 0 {
		t.Fatalf("CheckFastForward = %v, events %+v: main is an ancestor of the commit, so the fast-forward is possible", err, *got)
	}
	if strings.Join(f.calls, "\n") != fastForwardProbe+" [discard]" {
		t.Errorf("argv %q, want only the ancestry probe — nothing moves before the push", f.calls)
	}
}

func TestCheckFastForward_DivergedFleet_TransientRebaseNeeded_StampsStep(t *testing.T) {
	f := newFakeGit()
	f.on(fastForwardProbe, scripted{exit: 1})
	l, got := newLanding(f)

	err := l.CheckFastForward(context.Background(), integration(true))

	se := wantShipError(t, err, shiperr.CodeGitFleetRebaseNeeded, shiperr.ShipClassTransient,
		"ship: fleet ff-merge cycle-7-branch into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship")
	want := map[string]string{"git_rc": "1", "git_err": "", "cycle_branch": testCycleBr, "branch": testBranch, "step": "integrate"}
	if len(se.Debug) != len(want) {
		t.Errorf("Debug %v, want %v", se.Debug, want)
	}
	for k, v := range want {
		if se.Debug[k] != v {
			t.Errorf("Debug[%s]=%q, want %q", k, se.Debug[k], v)
		}
	}
	if len(*got) != 0 {
		t.Errorf("no warning on a divergence: %+v", *got)
	}
}

func TestCheckFastForward_DivergedNonFleet_PreconditionFFMergeDiverged(t *testing.T) {
	f := newFakeGit()
	f.on(fastForwardProbe, scripted{exit: 1})
	l, _ := newLanding(f)

	err := l.CheckFastForward(context.Background(), integration(false))

	se := wantShipError(t, err, shiperr.CodeGitFFMergeDiverged, shiperr.ShipClassPrecondition,
		"ship: ff-merge cycle-7-branch into main failed (rc=1; divergent history): <nil>")
	if se.Debug["step"] != "integrate" || se.Debug["cycle_branch"] != testCycleBr || se.Debug["git_rc"] != "1" {
		t.Errorf("Debug %v", se.Debug)
	}
}

func TestCheckFastForward_RunnerErrorCountsAsDivergence(t *testing.T) {
	f := newFakeGit()
	f.on(fastForwardProbe, scripted{err: errors.New("spawn: no git")})
	l, _ := newLanding(f)

	err := l.CheckFastForward(context.Background(), integration(false))

	se := wantShipError(t, err, shiperr.CodeGitFFMergeDiverged, shiperr.ShipClassPrecondition,
		"ship: ff-merge cycle-7-branch into main failed (rc=0; divergent history): spawn: no git")
	if se.Debug["git_err"] != "spawn: no git" || se.Debug["git_rc"] != "0" {
		t.Errorf("Debug %v", se.Debug)
	}
}
