package ship

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func TestHeldProposal_TheHoldRecordsItsBaseDiffAndDigestForTheAudit(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")

	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")

	rec, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
	diff, err := os.ReadFile(heldproposal.AuditDiffPath(f.evolveDir(), heldCycle))
	if err != nil || string(diff) != heldDiff {
		t.Fatalf("diff file = %q, %v, want the diff from the base sha to the commit", diff, err)
	}
	if rec.Base != heldBase || rec.DiffSHA256 != heldproposal.DigestOf([]byte(heldDiff)) {
		t.Errorf("record base %q digest %q, want the resolved base sha and the diff digest", rec.Base, rec.DiffSHA256)
	}
}

func TestHeldProposal_ARepeatedHoldKeepsTheBuilderFamiliesOfTheSameTree(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	f.audited(t, heldproposal.AuditWait)

	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")

	rec, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
	if len(rec.BuilderFamilies) != 1 || rec.BuilderFamilies[0] != "gemini" {
		t.Errorf("families = %v, want the attribution kept across the ship try", rec.BuilderFamilies)
	}
}

func TestHeldProposal_AnotherTreeOfTheSameCycleDoesNotInheritTheBuilderFamiliesOfTheFirst(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	f.audited(t, heldproposal.AuditWait)
	f.rec.set("rev-parse "+heldCommit+"^{tree}", scriptedCall{stdout: "5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d\n"})

	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")

	rec, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
	if rec.Tree != "5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d5ec0d" || len(rec.BuilderFamilies) != 0 {
		t.Errorf("record = %+v, want the new tree with no builder families: they belong to the tree that they described", rec)
	}
}

func TestHeldProposal_ABaseThatDoesNotResolveFailsTheHoldLoudly(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	f.rec.set("rev-parse main", scriptedCall{exit: 128})

	wantShipErr(t, f.check(), core.CodeStateIO, core.ShipClassTransient, "write the hold record")
}

func TestHeldProposal_AnApprovalWithAnAuditThatDoesNotPassIsRefusedNeverReheld(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		prepare func(f *heldFixture, t *testing.T)
		class   core.ShipErrorClass
		want    string
	}{
		"missing": {func(f *heldFixture, t *testing.T) {}, core.ShipClassPrecondition, "no adversarial audit report"},
		"failed":  {func(f *heldFixture, t *testing.T) { f.audited(t, heldproposal.AuditFail) }, core.ShipClassPrecondition, "did not pass"},
		"builder family": {func(f *heldFixture, t *testing.T) {
			f.audited(t, heldproposal.AuditPass)
			rec, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
			rec.BuilderFamilies = []string{"claude"}
			if err := heldproposal.WriteRecord(f.evolveDir(), rec); err != nil {
				t.Fatal(err)
			}
		}, core.ShipClassIntegrity, "built the change"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
			wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
			tc.prepare(f, t)
			if err := heldproposal.WriteApproval(f.evolveDir(), heldproposal.Approval{Cycle: heldCycle, Tree: heldTree, Decision: heldproposal.DecisionApprove}); err != nil {
				t.Fatal(err)
			}
			rec, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
			rec.Status = heldproposal.StatusApproved
			if err := heldproposal.WriteRecord(f.evolveDir(), rec); err != nil {
				t.Fatal(err)
			}

			wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, tc.class, tc.want)

			after, _, _ := heldproposal.ReadRecord(f.evolveDir(), heldCycle)
			if after.Status != heldproposal.StatusApproved {
				t.Errorf("status = %s, want the approved status kept: a failed audit check never rewrites the hold", after.Status)
			}
		})
	}
}

func heldShipErr() error {
	return core.NewShipError(shiperr.CodeHeldForOperator, core.ShipClassPrecondition, core.StageAtomicShip, "held", "cycle", "41")
}

func TestDispatchHeldAudit_CallsThePortAfterAHoldWithTheWorkspaceFallback(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	var gotWorkspace string
	var gotCycle int
	f.opts.HeldAudit = func(_ context.Context, _, workspace string, rec heldproposal.Record) error {
		gotWorkspace, gotCycle = workspace, rec.Cycle
		return errors.New("seat down")
	}
	res := &RunResult{}

	dispatchHeldAudit(context.Background(), f.opts, res, heldShipErr())

	if gotCycle != heldCycle || !strings.HasSuffix(gotWorkspace, "cycle-41") {
		t.Errorf("port got cycle %d workspace %q, want the cycle and the run workspace", gotCycle, gotWorkspace)
	}
	if len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "seat down") {
		t.Errorf("logs = %v, want the port error logged, never lost", res.Logs)
	}
}

func TestDispatchHeldAudit_NoPortRecordsAWaitReport(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")

	dispatchHeldAudit(context.Background(), f.opts, &RunResult{}, heldShipErr())

	report, _, found, err := heldproposal.ReadAuditReport(f.evolveDir(), heldCycle)
	if err != nil || !found || report.Verdict != heldproposal.AuditWait || !strings.HasPrefix(report.Reason, heldproposal.WaitPortNotWired) {
		t.Errorf("report = %+v, %v, %v, want a WAIT port_not_wired report", report, found, err)
	}
}

func TestDispatchHeldAudit_OnlyAPreconditionHoldOfAClassCycleShipDispatches(t *testing.T) {
	t.Parallel()
	calls := 0
	port := func(context.Context, string, string, heldproposal.Record) error { calls++; return nil }
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	f.opts.HeldAudit = port
	integrity := shipErr(shiperr.CodeHeldForOperator, core.ShipClassIntegrity, core.StageAtomicShip, "tamper", "cycle", "41")
	other := shipErr(core.CodeStateIO, core.ShipClassPrecondition, core.StageAtomicShip, "io", "cycle", "41")
	manual := *f.opts
	manual.Class = ClassManual

	for _, tc := range []struct {
		opts *Options
		err  error
	}{{f.opts, integrity}, {f.opts, other}, {f.opts, nil}, {&manual, heldShipErr()}, {f.opts, core.NewShipError(shiperr.CodeHeldForOperator, core.ShipClassPrecondition, core.StageAtomicShip, "held", "cycle", "x")}} {
		dispatchHeldAudit(context.Background(), tc.opts, &RunResult{}, tc.err)
	}

	if calls != 0 {
		t.Errorf("port called %d times, want 0 for a tamper, another code, no error, a manual ship or a bad cycle", calls)
	}
	f.opts.ProjectRoot = t.TempDir()
	res := &RunResult{}
	dispatchHeldAudit(context.Background(), f.opts, res, heldShipErr())
	if calls != 0 || len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "no hold record") {
		t.Errorf("calls %d logs %v, want no dispatch and a log when the hold record is absent", calls, res.Logs)
	}
}

func TestShipVerified_DispatchesTheHeldAuditAfterTheLandingWindowReturns(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	windowOpen, windowClosedAtDispatch := false, false
	f.opts.landWindow = func(context.Context, *Options, *RunResult) (RunResult, bool, error) {
		windowOpen = true
		defer func() { windowOpen = false }()
		return RunResult{}, false, heldShipErr()
	}
	dispatched := 0
	lockFreeAtDispatch, windowHeldAtDispatch := false, true
	f.opts.HeldAudit = func(ctx context.Context, _, _ string, _ heldproposal.Record) error {
		dispatched++
		windowClosedAtDispatch = !windowOpen
		windowHeldAtDispatch = shipwindow.Held(ctx)
		release, err := f.opts.acquireShipLock()
		if err == nil {
			release()
		}
		lockFreeAtDispatch = err == nil
		return nil
	}

	_, err := shipVerified(context.Background(), f.opts, &RunResult{})

	wantShipErr(t, err, shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	if dispatched != 1 || !windowClosedAtDispatch || !lockFreeAtDispatch || windowHeldAtDispatch {
		t.Errorf("dispatched %d, window closed %v, ship.lock free %v, window held in ctx %v, want one dispatch with no window and no ship.lock held", dispatched, windowClosedAtDispatch, lockFreeAtDispatch, windowHeldAtDispatch)
	}
}

func TestShipEntries_BothCarryThePortToTheSharedChokepoint(t *testing.T) {
	t.Parallel()
	port := func(context.Context, string, string, heldproposal.Record) error { return nil }
	phaseOpts := New(Config{HeldAudit: port}).shipOptions(core.PhaseRequest{ProjectRoot: t.TempDir(), Cycle: heldCycle}, "evolve-cycle 41")
	if phaseOpts.HeldAudit == nil {
		t.Error("Phase.Run drops Config.HeldAudit before the ship stages")
	}
	if New(Config{}).shipOptions(core.PhaseRequest{}, "m").HeldAudit != nil {
		t.Error("an unwired Config must leave the port nil, so the hold records a WAIT")
	}

	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	dispatched := 0
	f.opts.landWindow = func(context.Context, *Options, *RunResult) (RunResult, bool, error) {
		return RunResult{}, false, heldShipErr()
	}
	f.opts.HeldAudit = func(context.Context, string, string, heldproposal.Record) error { dispatched++; return nil }
	if _, err := shipVerified(context.Background(), f.opts, &RunResult{}); err == nil || dispatched != 1 {
		t.Errorf("shipVerified err %v dispatched %d, want the hold error and one dispatch: ship.Run and Phase.Run both end in shipVerified", err, dispatched)
	}
}

func TestHeldProposal_EachAuditInputFailureFailsTheHoldLoudly(t *testing.T) {
	t.Parallel()
	diff := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	diff.rec.set("diff "+heldBase+" "+heldCommit, scriptedCall{exit: 128})
	wantShipErr(t, diff.check(), core.CodeStateIO, core.ShipClassTransient, "write the hold record")

	corrupt := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	writeDoc(t, corrupt.opts.ProjectRoot, ".evolve/landing/held/cycle-41.json", "{not json")
	wantShipErr(t, corrupt.check(), core.CodeStateIO, core.ShipClassTransient, "write the hold record")

	blocked := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	if err := os.MkdirAll(heldproposal.AuditDiffPath(blocked.evolveDir(), heldCycle), 0o755); err != nil {
		t.Fatal(err)
	}
	wantShipErr(t, blocked.check(), core.CodeStateIO, core.ShipClassTransient, "write the hold record")
}

func TestShipVerified_ALandedShipRunsThePostShipHookAndFinishesWithoutAnAudit(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce")
	f.opts.landWindow = func(context.Context, *Options, *RunResult) (RunResult, bool, error) {
		return RunResult{}, true, nil
	}
	f.opts.HeldAudit = func(context.Context, string, string, heldproposal.Record) error {
		t.Error("a landed ship must not start an audit")
		return nil
	}
	f.opts.applyDefaults()

	out, err := shipVerified(context.Background(), f.opts, &RunResult{})

	if err != nil || out.ExitCode != ExitOK {
		t.Errorf("out %+v err %v, want a clean finish", out, err)
	}
}

func TestShipVerified_APostShipHookErrorIsLoggedAndTheLandedShipStillFinishes(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce")
	f.opts.landWindow = func(context.Context, *Options, *RunResult) (RunResult, bool, error) {
		return RunResult{}, true, nil
	}
	f.opts.applyDefaults()
	writeDoc(t, f.opts.ProjectRoot, ".evolve/cycle-state.json", "{not json")
	res := &RunResult{}

	_, err := shipVerified(context.Background(), f.opts, res)

	if err != nil || !strings.Contains(strings.Join(res.Logs, "\n"), "post-ship hook error") {
		t.Errorf("err %v logs %v, want the hook error logged and the ship finished", err, res.Logs)
	}
}

func TestDispatchHeldAudit_AnOuterShipWindowHeldByTheOrchestratorBlocksTheDispatchAndRecordsAWait(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	called := 0
	f.opts.HeldAudit = func(context.Context, string, string, heldproposal.Record) error { called++; return nil }
	lease, err := shipwindow.Acquire(context.Background(), f.evolveDir(), shipwindow.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()
	res := &RunResult{}

	dispatchHeldAudit(shipwindow.WithHeld(context.Background(), lease), f.opts, res, heldShipErr())

	report, _, found, rerr := heldproposal.ReadAuditReport(f.evolveDir(), heldCycle)
	if called != 0 || rerr != nil || !found || report.Verdict != heldproposal.AuditWait || !strings.HasPrefix(report.Reason, heldproposal.WaitShipWindowHeld) {
		t.Errorf("called %d report %+v found %v, want no dispatch and a ship_window_held WAIT", called, report, found)
	}
	if len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "holds the ship window") {
		t.Errorf("logs = %v, want the deferral logged", res.Logs)
	}

	dispatchHeldAudit(context.Background(), f.opts, &RunResult{}, heldShipErr())
	if called != 1 {
		t.Errorf("called %d, want the dispatch to run once the window is not held in the context", called)
	}
}

func TestDispatchHeldAudit_AFailedWaitRecordIsLogged(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "enforce", "go/internal/guards/role.go")
	wantShipErr(t, f.check(), shiperr.CodeHeldForOperator, core.ShipClassPrecondition, "")
	if err := os.MkdirAll(heldproposal.AuditReportPath(f.evolveDir(), heldCycle), 0o755); err != nil {
		t.Fatal(err)
	}
	res := &RunResult{}

	dispatchHeldAudit(context.Background(), f.opts, res, heldShipErr())

	if len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "record the wait") {
		t.Errorf("logs = %v, want the failed wait record logged, never lost", res.Logs)
	}
}

func shadowEvent(t *testing.T, f *heldFixture) (map[string]string, error) {
	t.Helper()
	err := f.check()
	if len(f.events) != 1 {
		t.Fatalf("events = %+v, want one SHIP_HOLD_SHADOW", f.events)
	}
	return f.events[0].Fields, err
}

func TestHeldProposal_ShadowRecordsTheAuditRouteItWouldTake(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "", "go/internal/guards/role.go")
	f.opts.HeldAuditPlan = func(workspace string, rec heldproposal.Record) heldaudit.Plan {
		if rec.Cycle != heldCycle || !strings.HasSuffix(workspace, "cycle-41") {
			t.Errorf("plan got cycle %d workspace %q", rec.Cycle, workspace)
		}
		return heldaudit.Plan{CLI: "agy-tmux", Family: "gemini", Tier: "deep"}
	}

	fields, err := shadowEvent(t, f)

	if err != nil || fields["audit_plan"] != "gemini/agy-tmux/deep" || fields["audit_wait"] != "" {
		t.Errorf("err %v fields %v, want the planned route and no wait", err, fields)
	}
}

func TestHeldProposal_ShadowRecordsTheWaitReasonAndStillLands(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "", "go/internal/guards/role.go")
	f.opts.HeldAuditPlan = func(string, heldproposal.Record) heldaudit.Plan {
		return heldaudit.Plan{Wait: heldproposal.WaitNoIndependentRoute, Detail: "only claude is left"}
	}

	fields, err := shadowEvent(t, f)

	if err != nil || fields["audit_plan"] != "" || fields["audit_wait"] != "no_independent_route: only claude is left" {
		t.Errorf("err %v fields %v, want the wait reason and a landing that goes on", err, fields)
	}
}

func TestHeldProposal_ShadowWithNoPlanPortRecordsPortNotWired(t *testing.T) {
	t.Parallel()
	f := newHeldFixture(t, "", "go/internal/guards/role.go")

	fields, err := shadowEvent(t, f)

	if err != nil || !strings.HasPrefix(fields["audit_wait"], heldproposal.WaitPortNotWired) {
		t.Errorf("err %v fields %v, want a port_not_wired wait and a landing that goes on", err, fields)
	}
	if _, err := os.Stat(heldproposal.AuditReportPath(f.evolveDir(), heldCycle)); err == nil {
		t.Error("shadow wrote an audit report: it only records a plan")
	}
}

func TestAuditWorkspace_AnExplicitWorkspaceWinsOverTheRunWorkspaceOfTheCycle(t *testing.T) {
	t.Parallel()
	explicit := &Options{ProjectRoot: "/root", WorkspacePath: "/ws/explicit"}
	fallback := &Options{ProjectRoot: "/root"}

	if got := auditWorkspace(explicit, 41); got != "/ws/explicit" {
		t.Errorf("explicit workspace = %q", got)
	}
	if got := auditWorkspace(fallback, 41); !strings.HasSuffix(got, "cycle-41") || !strings.HasPrefix(got, "/root") {
		t.Errorf("fallback workspace = %q, want the run workspace of the cycle under the root", got)
	}
}
