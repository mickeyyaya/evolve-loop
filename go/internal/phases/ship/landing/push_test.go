package landing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const repairLine = "[ship] REPAIR: push rejected — fetching origin and probing for a fast-forward retry"

const (
	worktreePush  = "push origin " + testHead + ":refs/heads/" + testBranch
	serverError   = "To github.com:o/r.git\n ! [remote rejected] main -> main (Internal Server Error)\nerror: failed to push some refs to 'github.com:o/r.git'\n"
	policyRefusal = " ! [remote rejected] main -> main (protected branch hook declined)\n"
	raceRejection = " ! [rejected]        main -> main (fetch first)\n"
)

func pushReq(site PushSite) PushRequest { return PushRequest{Branch: testBranch, Site: site} }

func worktreeReq() PushRequest {
	return PushRequest{Branch: testBranch, Commit: testHead, Site: SiteWorktree}
}

type sleeps []time.Duration

func (s *sleeps) record(d time.Duration) { *s = append(*s, d) }

func sleepingLanding(f *fakeGit) (*Landing, *[]signalcenter.Event, *sleeps) {
	c, got := recordingCenter()
	slept := &sleeps{}
	l := New(f.git, f.streams, WithRun("ship", 7, "r7"), WithSignals(func() *signalcenter.Center { return c }), WithSleep(slept.record))
	return l, got, slept
}

// rejectPush scripts the first push rejected (rc=1) and the second accepted.
func rejectPush(f *fakeGit) *fakeGit {
	return f.set("push origin "+testBranch, scripted{exit: 1}, scripted{exit: 0})
}

// declinedFixture provokes CodePushRepairDeclined on l (a failed fetch).
func declinedFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	rejectPush(f)
	f.set("fetch origin "+testBranch, scripted{exit: 1})
	if _, err := l.Push(context.Background(), pushReq(SiteDirect)); err == nil {
		t.Fatal("a declined repair returns the rejection")
	}
	f.set("push origin "+testBranch, scripted{})
	f.set("fetch origin "+testBranch, scripted{})
}

// headReadFailureFixture provokes CodeHeadReadFailed on l.
func headReadFailureFixture(t *testing.T, l *Landing, f *fakeGit) {
	t.Helper()
	f.set("rev-parse HEAD", scripted{exit: 128})
	if _, err := l.Push(context.Background(), pushReq(SiteDirect)); err != nil {
		t.Fatal(err)
	}
	f.set("rev-parse HEAD", scripted{stdout: testHead + "\n"})
}

func TestPush_RejectionWordingPerSite(t *testing.T) {
	rows := []struct {
		site  PushSite
		msg   string
		argv  []string
		debug map[string]string
	}{
		{SiteDirect, "ship: git push failed (rc=1)", []string{"push origin " + testBranch + " [streams]"},
			map[string]string{"git_rc": "1", "git_err": "", "branch": testBranch, "step": "push"}},
		{SiteWorktree, "ship: git push of " + testHead + " to origin/main failed (rc=1); ship did not move main", []string{worktreePush + " [streams]"},
			map[string]string{"git_rc": "1", "git_err": "", "branch": testBranch, "commit": testHead, "step": "push"}},
		{SitePushOnly, "ship --push-only: git push failed (rc=1)", []string{"push origin " + testBranch + " [streams]"},
			map[string]string{"git_rc": "1", "git_err": "", "branch": testBranch, "step": "push"}},
	}
	for _, row := range rows {
		f := newFakeGit()
		rejectPush(f)
		f.on(worktreePush, scripted{exit: 1})
		f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
		l, got := newLanding(f)
		req := pushReq(row.site)
		if row.site == SiteWorktree {
			req = worktreeReq()
		}
		req.RepairAttempted = true
		out, err := l.Push(context.Background(), req)
		se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient, row.msg)
		if len(se.Debug) != len(row.debug) {
			t.Errorf("site %d: Debug %v, want %v", row.site, se.Debug, row.debug)
		}
		for k, v := range row.debug {
			if se.Debug[k] != v {
				t.Errorf("site %d: Debug[%s]=%q, want %q", row.site, k, se.Debug[k], v)
			}
		}
		if want := row.argv; strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
			t.Errorf("site %d: argv %q, want %q", row.site, f.calls, want)
		}
		if out != (PushResult{}) || len(*got) != 0 {
			t.Errorf("site %d: the guarded path reports no repair and emits nothing: %+v %+v", row.site, out, *got)
		}
	}
}

func TestPush_RepairDeclinesOnEveryProbeFailure(t *testing.T) {
	rows := []struct {
		name   string
		probe  string
		script func(f *fakeGit)
	}{
		{"fetch rc=1", "fetch", func(f *fakeGit) { f.on("fetch origin "+testBranch, scripted{exit: 1}) }},
		{"fetch spawn error", "fetch", func(f *fakeGit) { f.on("fetch origin "+testBranch, scripted{err: errors.New("no git")}) }},
		{"origin ref unreadable", "origin_ref", func(f *fakeGit) { f.on("rev-parse origin/"+testBranch, scripted{exit: 128}) }},
		{"head unreadable", "head", func(f *fakeGit) {
			f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
			f.on("rev-parse HEAD", scripted{err: errors.New("no git")})
		}},
		{"retry push rc=1", "push_retry", func(f *fakeGit) {
			f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
			f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
			f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 0})
			f.set("push origin "+testBranch, scripted{exit: 1}, scripted{exit: 1})
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			f := newFakeGit()
			rejectPush(f)
			row.script(f)
			l, got := newLanding(f)
			sink, lines := logSink()
			req := pushReq(SiteDirect)
			req.Log = sink
			out, err := l.Push(context.Background(), req)
			se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient, "ship: git push failed (rc=1)")
			if se.Debug["repair_attempted"] != "GIT_PUSH_REJECTED" || se.Debug["repair_outcome"] != "declined" || se.Debug["step"] != "push" {
				t.Errorf("Debug %v", se.Debug)
			}
			if out != (PushResult{RepairAttempted: true, RepairOutcome: RepairDeclined}) {
				t.Errorf("result %+v", out)
			}
			if strings.Join(*lines, "\n") != repairLine {
				t.Errorf("log lines %q", *lines)
			}
			e := wantOneEvent(t, *got, CodePushRepairDeclined, "Landing.Push", map[string]string{"step": "push", "branch": testBranch, "probe": row.probe})
			if e.Reason != "the remote rejected the push; the inline repair declined at "+row.probe {
				t.Errorf("reason %q", e.Reason)
			}
		})
	}
}

// Test 17 — origin already at HEAD: already-pushed, nil error, the two
// REPAIR lines in order, no retry push, no warning, then the head read.
// Kills: push despite equality, the head read skipped.
func TestPush_RepairAlreadyPushed(t *testing.T) {
	f := newFakeGit()
	rejectPush(f)
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testHead + "\n"})
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	l, got := newLanding(f)
	sink, lines := logSink()
	req := pushReq(SiteDirect)
	req.Log = sink
	out, err := l.Push(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out != (PushResult{Head: testHead, RepairAttempted: true, RepairOutcome: RepairAlreadyPushed}) {
		t.Errorf("result %+v", out)
	}
	wantLogs := []string{repairLine, "[ship] REPAIR: origin already at HEAD — push race resolved itself"}
	if strings.Join(*lines, "\n") != strings.Join(wantLogs, "\n") {
		t.Errorf("log lines %q", *lines)
	}
	wantArgv := []string{"push origin main [streams]", "fetch origin main [discard]", "rev-parse origin/main [capture]", "rev-parse HEAD [capture]", "rev-parse HEAD [capture]"}
	if strings.Join(f.calls, "\n") != strings.Join(wantArgv, "\n") {
		t.Errorf("argv %q", f.calls)
	}
	if len(*got) != 0 {
		t.Errorf("no warning on a self-resolved race: %+v", *got)
	}
}

func TestPush_RepairFastForwardRetry_NeverRebasesNeverForcePushes(t *testing.T) {
	f := newFakeGit()
	rejectPush(f)
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 0})
	l, got := newLanding(f)
	sink, lines := logSink()
	req := pushReq(SiteDirect)
	req.Log = sink
	out, err := l.Push(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if out != (PushResult{Head: testHead, RepairAttempted: true, RepairOutcome: RepairPushRetried}) || string(out.RepairOutcome) != "push-retried" {
		t.Errorf("result %+v", out)
	}
	wantArgv := []string{"push origin main [streams]", "fetch origin main [discard]", "rev-parse origin/main [capture]",
		"rev-parse HEAD [capture]", "merge-base --is-ancestor " + testOriginRef + " HEAD [discard]", "push origin main [streams]", "rev-parse HEAD [capture]"}
	if strings.Join(f.calls, "\n") != strings.Join(wantArgv, "\n") {
		t.Errorf("argv %q", f.calls)
	}
	if (*lines)[len(*lines)-1] != "[ship] REPAIR: push retry after fetch succeeded (origin was an ancestor — fast-forward)" {
		t.Errorf("log lines %q", *lines)
	}
	if len(*got) != 0 {
		t.Errorf("no warning on a healed race: %+v", *got)
	}
	for _, a := range f.calls {
		for _, banned := range []string{"rebase", "--force", " -f ", "--force-with-lease"} {
			if strings.Contains(a, banned) {
				t.Errorf("the landing never rebases or force-pushes: %s", a)
			}
		}
	}
}

// Test 19 — origin diverged: needs-reaudit, a precondition GIT_PUSH_REJECTED
// with the :467-472 message and Debug{branch, origin_ref, head,
// repair_attempted, repair_outcome=needs-reaudit, step=push}; no push, no
// warning. Kills: class transient, repair_outcome missing, origin_ref
// untrimmed.
func TestPush_RepairDivergedReclassifiesNeedsReaudit(t *testing.T) {
	f := newFakeGit()
	rejectPush(f)
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 1})
	l, got := newLanding(f)
	out, err := l.Push(context.Background(), pushReq(SitePushOnly))
	se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition,
		"ship: push rejected and origin/main diverged — audited tree must be re-audited on the new base (no auto-rebase; local commit preserved). Reconcile at a batch boundary with `evolve sync-main`, then complete the stranded push with `evolve ship --push-only`.")
	want := map[string]string{"branch": testBranch, "origin_ref": testOriginRef, "head": testHead,
		"repair_attempted": "GIT_PUSH_REJECTED", "repair_outcome": "needs-reaudit", "step": "push"}
	if len(se.Debug) != len(want) {
		t.Errorf("Debug %v, want %v", se.Debug, want)
	}
	for k, v := range want {
		if se.Debug[k] != v {
			t.Errorf("Debug[%s]=%q, want %q", k, se.Debug[k], v)
		}
	}
	if out != (PushResult{RepairAttempted: true, RepairOutcome: RepairNeedsReaudit}) {
		t.Errorf("result %+v", out)
	}
	if strings.Count(strings.Join(f.calls, "\n"), "push origin") != 1 || len(*got) != 0 {
		t.Errorf("no retry push and no warning on a divergence: %v %+v", f.calls, *got)
	}
}

// Test 20 — the once-guard and --dry-run skip the repair: the rejection is
// returned, zero probes, PushResult{false, ""}. Kills: guard ignored, DryRun
// repaired, RepairAttempted reported on the guarded path.
func TestPush_OnceGuardAndDryRunSkipTheRepair(t *testing.T) {
	for name, req := range map[string]PushRequest{
		"once-guard": {Branch: testBranch, Site: SiteDirect, RepairAttempted: true},
		"dry-run":    {Branch: testBranch, Site: SiteDirect, DryRun: true},
	} {
		f := newFakeGit()
		rejectPush(f)
		l, got := newLanding(f)
		sink, lines := logSink()
		req.Log = sink
		out, err := l.Push(context.Background(), req)
		se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient, "")
		if _, stamped := se.Debug["repair_outcome"]; stamped {
			t.Errorf("%s: the guarded path never stamps a repair outcome: %v", name, se.Debug)
		}
		if out != (PushResult{}) || len(f.calls) != 1 || len(*lines) != 0 || len(*got) != 0 {
			t.Errorf("%s: zero probes, no result, no log, no warning: %+v %v %v %+v", name, out, f.calls, *lines, *got)
		}
	}
}

// Test 21 — after the push landed, an unreadable or empty `rev-parse HEAD`
// leaves Head empty, returns nil and emits exactly ONE
// SHIP_LANDING_HEAD_READ_FAILED{step=push, ref=HEAD, err}. Kills: an error
// returned instead of proceeding, a warning on a readable HEAD.
func TestPush_HeadReadFailureWarnsAndLeavesHeadEmpty(t *testing.T) {
	for name, call := range map[string]scripted{"unreadable": {exit: 128}, "empty": {stdout: ""}} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGit()
			f.on("rev-parse HEAD", call)
			l, got := newLanding(f)
			out, err := l.Push(context.Background(), pushReq(SiteDirect))
			if err != nil || out.Head != "" {
				t.Fatalf("the ship proceeds with an empty head: %+v %v", out, err)
			}
			e := wantOneEvent(t, *got, CodeHeadReadFailed, "Landing.Push", map[string]string{"step": "push", "ref": "HEAD"})
			if name == "unreadable" && (e.Fields["err"] == "" || !strings.HasPrefix(e.Reason, "rev-parse HEAD after the push failed: ")) {
				t.Errorf("reason %q fields %v", e.Reason, e.Fields)
			}
			if name == "empty" && e.Reason != "rev-parse HEAD after the push returned empty" {
				t.Errorf("reason %q", e.Reason)
			}
		})
	}
	f := newFakeGit()
	f.on("rev-parse HEAD", scripted{stdout: " " + testHead + "\n"})
	l, got := newLanding(f)
	out, err := l.Push(context.Background(), pushReq(SiteDirect))
	if err != nil || out.Head != testHead || len(*got) != 0 {
		t.Errorf("a readable HEAD is trimmed and silent: %+v %v %+v", out, err, *got)
	}
}

// Test 23 — IsAncestor is true only on rc 0 without an error; the argv is
// `merge-base --is-ancestor a b` to io.Discard. Kills: the error ignored.
func TestIsAncestor_TrueOnlyOnRcZeroWithoutError(t *testing.T) {
	f := newFakeGit()
	f.on("merge-base --is-ancestor a b", scripted{exit: 0}, scripted{exit: 1}, scripted{err: errors.New("no git")})
	l, _ := newLanding(f)
	for i, want := range []bool{true, false, false} {
		if got := l.IsAncestor(context.Background(), "a", "b"); got != want {
			t.Errorf("call %d: %v, want %v", i, got, want)
		}
	}
	if f.calls[0] != "merge-base --is-ancestor a b [discard]" {
		t.Errorf("argv %q", f.calls[0])
	}
}

// Test 24 — Capture: a spawn error and rc > 1 are GIT_IO with the two
// message texts and Debug{git_args, git_err} / {git_args, git_rc}; rc=1 is
// success; no step (the caller's step is unknown here). Kills: > 1 → >= 1,
// message drift.
func TestCapture_ExitRuleAndErrorTexts(t *testing.T) {
	f := newFakeGit()
	f.on("rev-parse HEAD", scripted{err: errors.New("boom")}, scripted{exit: 2}, scripted{stdout: "out\n", exit: 1})
	l, _ := newLanding(f)
	_, err := l.Capture(context.Background(), "rev-parse", "HEAD")
	se := wantShipError(t, err, shiperr.CodeGitIO, shiperr.ShipClassTransient, "ship: git [rev-parse HEAD]: boom")
	if se.Debug["git_args"] != "[rev-parse HEAD]" || se.Debug["git_err"] != "boom" || len(se.Debug) != 2 {
		t.Errorf("spawn error Debug %v", se.Debug)
	}
	_, err = l.Capture(context.Background(), "rev-parse", "HEAD")
	se = wantShipError(t, err, shiperr.CodeGitIO, shiperr.ShipClassTransient, "ship: git [rev-parse HEAD] exited 2")
	if se.Debug["git_args"] != "[rev-parse HEAD]" || se.Debug["git_rc"] != "2" || len(se.Debug) != 2 {
		t.Errorf("rc=2 Debug %v", se.Debug)
	}
	out, err := l.Capture(context.Background(), "rev-parse", "HEAD")
	if err != nil || out != "out\n" {
		t.Errorf("rc=1 is success: %q %v", out, err)
	}
	if f.calls[0] != "rev-parse HEAD [capture]" {
		t.Errorf("a capture streams stdout to a builder and stderr to io.Discard: %q", f.calls[0])
	}
}

func TestPush_TheWorktreeSitePushesTheCommitByRefspecAndProbesTheRaceAgainstIt(t *testing.T) {
	f := newFakeGit()
	f.set(worktreePush, scripted{exit: 1, stderr: raceRejection}, scripted{exit: 0})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 0})
	l, got, slept := sleepingLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	if err != nil || out != (PushResult{Head: testHead, RepairAttempted: true, RepairOutcome: RepairPushRetried}) {
		t.Fatalf("Push = %+v, %v; want the commit landed by the race repair", out, err)
	}
	want := []string{worktreePush + " [streams]", "fetch origin main [discard]", "rev-parse origin/main [capture]",
		"merge-base --is-ancestor " + testOriginRef + " " + testHead + " [discard]", worktreePush + " [streams]"}
	if strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("argv\n got %q\nwant %q: the worktree site never reads HEAD, because main has not moved", f.calls, want)
	}
	if len(*slept) != 0 || len(*got) != 0 {
		t.Errorf("slept %v, events %+v: a race is probed, never backed off", *slept, *got)
	}
}

func TestPush_TheWorktreeSiteDivergenceNamesTheCommitAndNeedsAReaudit(t *testing.T) {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: raceRejection})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" "+testHead, scripted{exit: 1})
	l, _ := newLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassPrecondition,
		"ship: origin rejected the push of "+testHead+" because origin/main diverged; Audit must run again on the new base (ship does not rebase or force-push)")
	if se.Debug["head"] != testHead || se.Debug["repair_outcome"] != "needs-reaudit" || out.RepairOutcome != RepairNeedsReaudit {
		t.Errorf("Debug %v, result %+v", se.Debug, out)
	}
}

func TestPush_ATransportErrorBacksOffAndRetriesTheIdenticalPush(t *testing.T) {
	f := newFakeGit()
	f.set(worktreePush, scripted{exit: 1, stderr: serverError}, scripted{exit: 1, stderr: serverError}, scripted{exit: 0})
	l, got, slept := sleepingLanding(f)
	sink, lines := logSink()
	req := worktreeReq()
	req.Log = sink

	out, err := l.Push(context.Background(), req)

	if err != nil || out != (PushResult{Head: testHead, RepairAttempted: true, RepairOutcome: RepairPushRetried}) {
		t.Fatalf("Push = %+v, %v; want the third identical push to land", out, err)
	}
	if want := (sleeps{2 * time.Second, 4 * time.Second}); strings.Join(durations(*slept), ",") != strings.Join(durations(want), ",") {
		t.Errorf("slept %v, want %v", *slept, want)
	}
	if want := []string{worktreePush + " [streams]", worktreePush + " [streams]", worktreePush + " [streams]"}; strings.Join(f.calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("argv %q, want three identical pushes and no fetch probe", f.calls)
	}
	wantLogs := []string{"[ship] REPAIR: the push failed with a transport or server error; ship retries the identical push after a backoff", "[ship] REPAIR: push retry after a 4s backoff succeeded"}
	if strings.Join(*lines, "\n") != strings.Join(wantLogs, "\n") || len(*got) != 0 {
		t.Errorf("logs %q events %+v", *lines, *got)
	}
}

func TestPush_ATransportOutageDeclinesAfterTheBoundedBackoff(t *testing.T) {
	f := newFakeGit()
	f.set(worktreePush, scripted{exit: 1, stderr: serverError})
	l, got, slept := sleepingLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient,
		"ship: git push of "+testHead+" to origin/main failed (rc=1): To github.com:o/r.git | ! [remote rejected] main -> main (Internal Server Error) | error: failed to push some refs to 'github.com:o/r.git'; ship did not move main")
	if se.Debug["repair_outcome"] != "declined" || out != (PushResult{RepairAttempted: true, RepairOutcome: RepairDeclined}) {
		t.Errorf("Debug %v result %+v", se.Debug, out)
	}
	if want := (sleeps{2 * time.Second, 4 * time.Second, 8 * time.Second}); strings.Join(durations(*slept), ",") != strings.Join(durations(want), ",") || len(f.calls) != 4 {
		t.Errorf("slept %v over %d pushes, want %v over 4: the backoff is bounded inside the step", *slept, len(f.calls), want)
	}
	wantOneEvent(t, *got, CodePushRepairDeclined, "Landing.Push", map[string]string{"step": "push", "branch": testBranch, "probe": "transport_retry"})
}

func TestPush_ARejectionThatStopsBeingTransportEndsTheBackoff(t *testing.T) {
	f := newFakeGit()
	f.set(worktreePush, scripted{exit: 1, stderr: serverError}, scripted{exit: 1, stderr: raceRejection})
	l, _, slept := sleepingLanding(f)

	_, err := l.Push(context.Background(), worktreeReq())

	wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient, "")
	if len(*slept) != 1 || len(f.calls) != 2 {
		t.Errorf("slept %v over %d pushes, want one backoff and two pushes: the backoff never applies to a race", *slept, len(f.calls))
	}
}

func TestPush_APolicyRefusalIsFinalAtEverySite(t *testing.T) {
	for name, req := range map[string]PushRequest{"direct": pushReq(SiteDirect), "worktree": worktreeReq(), "once-guarded": {Branch: testBranch, Site: SitePushOnly, RepairAttempted: true}} {
		t.Run(name, func(t *testing.T) {
			f := newFakeGit()
			f.on("push origin "+testBranch, scripted{exit: 1, stderr: policyRefusal})
			f.on(worktreePush, scripted{exit: 1, stderr: policyRefusal})
			l, got, slept := sleepingLanding(f)

			out, err := l.Push(context.Background(), req)

			se := wantShipError(t, err, shiperr.CodeGitPushPolicyRefused, shiperr.ShipClassPrecondition,
				"ship: origin refused the push to main by policy; ship does not retry a policy refusal: ! [remote rejected] main -> main (protected branch hook declined)")
			if se.Debug["step"] != "push" || se.Debug["git_rc"] != "1" {
				t.Errorf("Debug %v", se.Debug)
			}
			if len(f.calls) != 1 || len(*slept) != 0 || out != (PushResult{}) || len(*got) != 0 {
				t.Errorf("calls %q slept %v result %+v events %+v: a policy refusal makes no probe, no retry and no backoff", f.calls, *slept, out, *got)
			}
		})
	}
}

func TestPush_TheOperatorStderrStillReceivesThePushOutput(t *testing.T) {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: policyRefusal})
	l, _ := newLanding(f)

	_, err := l.Push(context.Background(), worktreeReq())

	if f.stderr.String() != policyRefusal || err == nil || !strings.Contains(err.Error(), "protected branch hook declined") {
		t.Errorf("operator stderr %q, error %v: git's stderr reaches the operator and the classifier both", f.stderr.String(), err)
	}
}

func TestPush_WithoutAnOperatorStderrTheRejectionIsStillClassified(t *testing.T) {
	f := newFakeGit()
	f.on(worktreePush, scripted{exit: 1, stderr: policyRefusal})
	l := New(f.git, func() Streams { return Streams{Stdout: &f.stdout} })

	_, err := l.Push(context.Background(), worktreeReq())

	wantShipError(t, err, shiperr.CodeGitPushPolicyRefused, shiperr.ShipClassPrecondition, "")
}

func durations(d sleeps) []string {
	out := make([]string, len(d))
	for i, v := range d {
		out[i] = v.String()
	}
	return out
}

func TestPush_ASpawnErrorIsNamedInTheRejectionInsteadOfItsStderr(t *testing.T) {
	f := newFakeGit()
	f.on("push origin "+testBranch, scripted{err: errors.New("spawn: no git"), stderr: serverError})
	l, _ := newLanding(f)
	req := pushReq(SiteDirect)
	req.RepairAttempted = true

	_, err := l.Push(context.Background(), req)

	se := wantShipError(t, err, shiperr.CodeGitPushRejected, shiperr.ShipClassTransient, "ship: git push failed (rc=0): spawn: no git")
	if se.Debug["git_err"] != "spawn: no git" {
		t.Errorf("Debug %v", se.Debug)
	}
}

func TestPush_APolicyRefusalDuringTheBackoffIsFinalNotTransient(t *testing.T) {
	f := newFakeGit()
	f.set(worktreePush, scripted{exit: 1, stderr: serverError}, scripted{exit: 1, stderr: policyRefusal})
	l, _, slept := sleepingLanding(f)

	out, err := l.Push(context.Background(), worktreeReq())

	wantShipError(t, err, shiperr.CodeGitPushPolicyRefused, shiperr.ShipClassPrecondition,
		"ship: origin refused the push to main by policy; ship does not retry a policy refusal: ! [remote rejected] main -> main (protected branch hook declined)")
	if len(*slept) != 1 || len(f.calls) != 2 || out.RepairOutcome != RepairDeclined {
		t.Errorf("slept %v over %d pushes, result %+v: a refusal after one backoff ends the retries as a policy precondition", *slept, len(f.calls), out)
	}
}

func TestPush_AWorktreeSiteWithNoCommitNeverPushes(t *testing.T) {
	f := newFakeGit()
	l, _ := newLanding(f)

	_, err := l.Push(context.Background(), PushRequest{Branch: testBranch, Site: SiteWorktree})

	wantShipError(t, err, shiperr.CodeArgs, shiperr.ShipClassConfig, "ship: a worktree push names no commit, so ship pushes nothing")
	if len(f.calls) != 0 {
		t.Errorf("calls %q: a refspec with no commit deletes the remote branch, so ship runs no git", f.calls)
	}
}
