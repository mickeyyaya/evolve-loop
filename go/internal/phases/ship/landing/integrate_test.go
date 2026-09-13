package landing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func integration(fleet bool) Integration {
	return Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve", Fleet: fleet}
}

// resetFailureFixture provokes CodeBinaryResetFailed on l.
func resetFailureFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	f.set("checkout HEAD -- go/evolve", scripted{exit: 1})
	if err := l.Integrate(context.Background(), integration(false)); err != nil {
		t.Fatal(err)
	}
	f.set("checkout HEAD -- go/evolve", scripted{})
}

// Test 10 — the reset runs first to io.Discard, the merge second to the
// operator streams, the OK line is logged verbatim. Kills: argv order
// swapped, the merge streams discarded, the OK line text drifted.
func TestIntegrate_ResetThenFFMerge_ArgvOrderAndStreams(t *testing.T) {
	f := newFakeGit()
	l, got := newLanding(f)
	sink, lines := logSink()
	req := integration(false)
	req.Log = sink
	if err := l.Integrate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	want := []string{"checkout HEAD -- go/evolve [discard]", "merge --ff-only " + testCycleBr + " [streams]"}
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

// Test 11 — a failed reset (rc=1, or a spawn error) is ONE ship.warning
// SHIP_LANDING_BINARY_RESET_FAILED from Landing.Integrate with step/path/
// git_rc/git_err; the merge still runs; nothing is written to Stderr.
// Kills: return on reset failure, the origin typo, fields dropped, the
// Fprintf kept.
func TestIntegrate_ResetFailure_EmitsBinaryResetFailedAndProceeds(t *testing.T) {
	for name, call := range map[string]scripted{"rc=1": {exit: 1}, "spawn error": {err: errors.New("no git")}} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGit()
			f.on("checkout HEAD -- go/evolve", call)
			l, got := newLanding(f)
			if err := l.Integrate(context.Background(), integration(false)); err != nil {
				t.Fatal(err)
			}
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

// Test 12 — a diverged merge under the fleet is GIT_FLEET_REBASE_NEEDED /
// transient with the :174 message and Debug{git_rc, git_err, cycle_branch,
// branch, step=integrate}; no OK line. Kills: class → precondition, code
// swapped, message drift, step missing.
func TestIntegrate_DivergedFleet_TransientRebaseNeeded_StampsStep(t *testing.T) {
	f := newFakeGit()
	f.on("merge --ff-only "+testCycleBr, scripted{exit: 128})
	l, got := newLanding(f)
	sink, lines := logSink()
	req := integration(true)
	req.Log = sink
	err := l.Integrate(context.Background(), req)
	se := wantShipError(t, err, shiperr.CodeGitFleetRebaseNeeded, shiperr.ShipClassTransient,
		"ship: fleet ff-merge cycle-7-branch into main diverged (a peer cycle moved main mid-pipeline); rebase + re-verify the merged tree, then re-ship")
	want := map[string]string{"git_rc": "128", "git_err": "", "cycle_branch": testCycleBr, "branch": testBranch, "step": "integrate"}
	if len(se.Debug) != len(want) {
		t.Errorf("Debug %v, want %v", se.Debug, want)
	}
	for k, v := range want {
		if se.Debug[k] != v {
			t.Errorf("Debug[%s]=%q, want %q", k, se.Debug[k], v)
		}
	}
	if len(*lines) != 0 || len(*got) != 0 {
		t.Errorf("no OK line and no warning on a divergence: %v %+v", *lines, *got)
	}
}

// Test 13 — outside the fleet the same divergence is GIT_FF_MERGE_DIVERGED /
// precondition with the :178 message and step. Kills: Fleet ignored.
func TestIntegrate_DivergedNonFleet_PreconditionFFMergeDiverged(t *testing.T) {
	f := newFakeGit()
	f.on("merge --ff-only "+testCycleBr, scripted{exit: 128})
	l, _ := newLanding(f)
	err := l.Integrate(context.Background(), integration(false))
	se := wantShipError(t, err, shiperr.CodeGitFFMergeDiverged, shiperr.ShipClassPrecondition,
		"ship: ff-merge cycle-7-branch into main failed (rc=128; divergent history): <nil>")
	if se.Debug["step"] != "integrate" || se.Debug["cycle_branch"] != testCycleBr || se.Debug["git_rc"] != "128" {
		t.Errorf("Debug %v", se.Debug)
	}
}

// Test 14 — a spawn error with exit 0 is a divergence too (err != nil ||
// exit != 0). Kills: || → &&.
func TestIntegrate_RunnerErrorCountsAsDivergence(t *testing.T) {
	f := newFakeGit()
	f.on("merge --ff-only "+testCycleBr, scripted{err: errors.New("spawn: no git")})
	l, _ := newLanding(f)
	err := l.Integrate(context.Background(), integration(false))
	se := wantShipError(t, err, shiperr.CodeGitFFMergeDiverged, shiperr.ShipClassPrecondition,
		"ship: ff-merge cycle-7-branch into main failed (rc=0; divergent history): spawn: no git")
	if se.Debug["git_err"] != "spawn: no git" || se.Debug["git_rc"] != "0" {
		t.Errorf("Debug %v", se.Debug)
	}
}
