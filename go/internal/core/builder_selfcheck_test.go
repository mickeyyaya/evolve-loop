package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	selfCheckCoverFinding = "cover-changed: go/internal/x/x.go:Foo is at 80.0%, below 100%"
	selfCheckAreaFinding  = "explanation-documentation: Changed Areas omits go/internal/x/x.go"
)

func selfCheckWorktree(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("x.go", "package x\n")
	repo.Git("add", "x.go")
	return repo.Dir
}

func writeSelfCheckFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type countingFloor struct {
	calls    atomic.Int32
	findings []string
}

func (f *countingFloor) floor() BuildHandoffFloor {
	return BuildHandoffFloor{{Name: "fixture", Run: func(context.Context, ReviewInput) []string {
		f.calls.Add(1)
		return f.findings
	}}}
}

func newSelfCheck(t *testing.T, stage config.Stage, floor *countingFloor) (builderSelfCheck, *[]signalcenter.Event) {
	t.Helper()
	center := signalcenter.New()
	var got []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	in := ReviewInput{Cycle: 2092, Phase: string(PhaseBuild), Workspace: t.TempDir(), Worktree: selfCheckWorktree(t)}
	return builderSelfCheck{stage: stage, floor: floor.floor(), in: in, signals: center, errOut: io.Discard}, &got
}

func readSelfCheckFindings(t *testing.T, workspace string) selfCheckFindings {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspace, BuilderSelfCheckFindingsFile))
	if err != nil {
		t.Fatalf("findings file: %v", err)
	}
	var rec selfCheckFindings
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestBuilderSelfCheck_EnforceWithNoReceiptReturnsExactlyTheFloorFindings(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding, selfCheckAreaFinding}}
	check, events := newSelfCheck(t, config.StageEnforce, floor)

	msg, report := check.beforeClose(context.Background())

	if floor.calls.Load() != 1 {
		t.Fatalf("floor ran %d times, want 1", floor.calls.Load())
	}
	for _, f := range floor.findings {
		if !strings.Contains(msg, "\n  - "+f) {
			t.Errorf("message lacks the finding %q:\n%s", f, msg)
		}
	}
	if strings.Count(msg, "\n  - ") != 2 || !strings.Contains(msg, "evolve selfcheck build") {
		t.Errorf("message must list exactly the two findings and name the self-check verb:\n%s", msg)
	}
	if len(*events) != 0 {
		t.Fatalf("a record before the paste claims an outcome that has not happened: %+v", *events)
	}
	report(nil)
	rec := readSelfCheckFindings(t, check.in.Workspace)
	if rec.Decision != selfCheckDelivered || rec.Stage != "enforce" || rec.Source != selfCheckSourceBridge || rec.Receipt != receiptAbsent || len(rec.Findings) != 2 || rec.Error != "" {
		t.Errorf("findings record = %+v", rec)
	}
	if len(*events) != 1 || (*events)[0].Code != CodeBuilderSelfCheck || (*events)[0].Fields["decision"] != selfCheckDelivered || (*events)[0].Cycle != 2092 {
		t.Errorf("events = %+v", *events)
	}
}

func TestBuilderSelfCheck_AFailedPasteIsRecordedAsDeliveryFailedWithTheError(t *testing.T) {
	t.Parallel()
	check, events := newSelfCheck(t, config.StageEnforce, &countingFloor{findings: []string{selfCheckCoverFinding}})

	_, report := check.beforeClose(context.Background())
	report(errors.New("paste refused"))

	rec := readSelfCheckFindings(t, check.in.Workspace)
	if rec.Decision != selfCheckDeliveryFailed || rec.Error != "paste refused" {
		t.Errorf("findings record = %+v", rec)
	}
	if len(*events) != 1 || (*events)[0].Fields["decision"] != selfCheckDeliveryFailed || (*events)[0].Fields["error"] != "paste refused" {
		t.Errorf("events = %+v", *events)
	}
}

func TestBuilderSelfCheck_AGreenReceiptForTheCurrentTreeSkipsTheFloor(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding}}
	check, events := newSelfCheck(t, config.StageEnforce, floor)
	if err := WriteSelfCheckReceipt(context.Background(), check.in.Workspace, check.in.Worktree, true); err != nil {
		t.Fatal(err)
	}

	if msg, report := check.beforeClose(context.Background()); msg != "" || report != nil || floor.calls.Load() != 0 || len(*events) != 0 {
		t.Fatalf("a GREEN receipt for the tree: msg %q, floor calls %d, events %d; want no delivery and no floor run", msg, floor.calls.Load(), len(*events))
	}
}

func TestBuilderSelfCheck_AReceiptForAnotherTreeCountsAsNoReceipt(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding}}
	check, _ := newSelfCheck(t, config.StageEnforce, floor)
	if err := WriteSelfCheckReceipt(context.Background(), check.in.Workspace, check.in.Worktree, true); err != nil {
		t.Fatal(err)
	}
	writeSelfCheckFile(t, check.in.Worktree, "y.go", "package x\n")

	msg, report := check.beforeClose(context.Background())
	if !strings.Contains(msg, selfCheckCoverFinding) || floor.calls.Load() != 1 {
		t.Fatalf("a receipt for another tree: msg %q, floor calls %d; want the findings", msg, floor.calls.Load())
	}
	report(nil)
	if rec := readSelfCheckFindings(t, check.in.Workspace); rec.Receipt != receiptStale {
		t.Errorf("receipt state = %q, want stale", rec.Receipt)
	}
}

func TestBuilderSelfCheck_TheReceiptStateNamesEachCase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	worktree, workspace := selfCheckWorktree(t), t.TempDir()
	if got := selfCheckReceiptState(ctx, workspace, worktree); got != receiptAbsent {
		t.Errorf("no receipt: %q", got)
	}
	if err := WriteSelfCheckReceipt(ctx, workspace, worktree, false); err != nil {
		t.Fatal(err)
	}
	if got := selfCheckReceiptState(ctx, workspace, worktree); got != receiptRed {
		t.Errorf("a RED receipt for the tree: %q", got)
	}
	if err := WriteSelfCheckReceipt(ctx, workspace, worktree, true); err != nil {
		t.Fatal(err)
	}
	if got := selfCheckReceiptState(ctx, workspace, worktree); got != receiptGreen {
		t.Errorf("a GREEN receipt for the tree: %q", got)
	}
	if got := selfCheckReceiptState(ctx, workspace, filepath.Join(t.TempDir(), "gone")); got != receiptStale {
		t.Errorf("a tree that cannot be read: %q", got)
	}
	writeSelfCheckFile(t, workspace, SelfCheckReceiptFile, "{")
	if got := selfCheckReceiptState(ctx, workspace, worktree); got != receiptAbsent {
		t.Errorf("an unparseable receipt: %q", got)
	}
	if err := WriteSelfCheckReceipt(ctx, workspace, "", true); err == nil {
		t.Error("a receipt with no worktree must refuse, because it binds no tree")
	}
	if err := WriteSelfCheckReceipt(ctx, filepath.Join(workspace, SelfCheckReceiptFile, "sub"), worktree, true); err == nil {
		t.Error("an unwritable workspace must return the write error")
	}
}

func TestBuilderSelfCheck_NoFindingsOrAnExpiredContextDeliversNothing(t *testing.T) {
	t.Parallel()
	check, events := newSelfCheck(t, config.StageEnforce, &countingFloor{})
	if msg, report := check.beforeClose(context.Background()); msg != "" || report != nil {
		t.Fatalf("a clean floor: msg %q", msg)
	}
	late, _ := newSelfCheck(t, config.StageEnforce, &countingFloor{findings: []string{selfCheckCoverFinding}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if msg, report := late.beforeClose(ctx); msg != "" || report != nil {
		t.Fatalf("a floor that ran past the deadline of the wait: msg %q", msg)
	}
	if len(*events) != 0 {
		t.Errorf("events = %+v", *events)
	}
}

func TestBuilderSelfCheck_AnUnwritableFindingsFileStillSignals(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding}}
	check, events := newSelfCheck(t, config.StageEnforce, floor)
	writeSelfCheckFile(t, check.in.Workspace, "not-a-dir", "")
	check.in.Workspace = filepath.Join(check.in.Workspace, "not-a-dir")
	var warn bytes.Buffer
	check.errOut = &warn

	msg, report := check.beforeClose(context.Background())
	report(nil)

	if !strings.Contains(msg, selfCheckCoverFinding) || len(*events) != 1 {
		t.Fatalf("msg %q events %d; the signal and the delivery do not depend on the file", msg, len(*events))
	}
	if !strings.Contains(warn.String(), "[builder-selfcheck] WARN findings record not written") {
		t.Errorf("a failed findings write must warn on the orchestrator output: %q", warn.String())
	}
}

func TestBuilderSelfCheckBeforeClose_OnlyAnEnforcedBuildWithAFloorRunsTheFloorInTheWait(t *testing.T) {
	t.Parallel()
	floor := (&countingFloor{}).floor()
	run := func(phase Phase, stage config.Stage, reviewer DeliverableReviewer) CloseCheck {
		opts := []Option{WithWorkflowConfig(policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: stage}})}
		if reviewer != nil {
			opts = append(opts, WithReviewer(reviewer))
		}
		cr := &cycleRun{o: NewOrchestrator(nil, nil, nil, opts...), cs: CycleState{CycleID: 7}}
		return cr.builderSelfCheckBeforeClose(phase)
	}
	if run(PhaseBuild, config.StageEnforce, floor) == nil {
		t.Error("a build at enforce with a floor gets the check")
	}
	if run(PhaseBuild, config.StageShadow, floor) != nil || run(PhaseBuild, config.StageOff, floor) != nil {
		t.Error("shadow and off add no floor run to the wait")
	}
	if run(PhaseAudit, config.StageEnforce, floor) != nil {
		t.Error("a phase other than build gets no check")
	}
	bare := &cycleRun{o: &Orchestrator{workflowConfig: policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: config.StageEnforce}}}}
	if bare.builderSelfCheckBeforeClose(PhaseBuild) != nil {
		t.Error("an orchestrator with no reviewer has no handoff floor and gets no check")
	}
}

type inSessionBuilder struct {
	*fakeRunner
	fixed       *atomic.Bool
	inRun       *atomic.Bool
	delivered   []string
	workspace   string
	pasteFailed bool
}

func (b *inSessionBuilder) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	b.workspace = req.Workspace
	b.inRun.Store(true)
	defer b.inRun.Store(false)
	if req.BeforeClose != nil {
		if msg, report := req.BeforeClose(ctx); msg != "" {
			b.delivered = append(b.delivered, msg)
			if b.pasteFailed {
				report(errors.New("paste refused"))
			} else {
				report(nil)
				b.fixed.Store(true)
			}
		}
	}
	if req.CorrectionDirective != "" {
		b.fixed.Store(true)
	}
	return b.fakeRunner.Run(ctx, req)
}

type selfCheckCycle struct {
	builder     *inSessionBuilder
	floorInWait int32
	floorAtGate int32
	err         error
}

func runSelfCheckCycle(t *testing.T, stage config.Stage, pasteFailed bool) selfCheckCycle {
	t.Helper()
	fixed, inRun := &atomic.Bool{}, &atomic.Bool{}
	var inWait, atGate atomic.Int32
	floor := BuildHandoffFloor{{Name: "fixture", Run: func(context.Context, ReviewInput) []string {
		if inRun.Load() {
			inWait.Add(1)
		} else {
			atGate.Add(1)
		}
		if fixed.Load() {
			return nil
		}
		return []string{selfCheckCoverFinding}
	}}}
	runners := buildRunners(nil)
	builder := &inSessionBuilder{fakeRunner: runners[PhaseBuild].(*fakeRunner), fixed: fixed, inRun: inRun, pasteFailed: pasteFailed}
	runners[PhaseBuild] = builder
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithReviewer(floor),
		WithWorkflowConfig(policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: stage}}))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	return selfCheckCycle{builder: builder, floorInWait: inWait.Load(), floorAtGate: atGate.Load(), err: err}
}

func TestBuilderSelfCheck_EnforceFixesTheFloorInTheSameSessionWithNoNewDispatch(t *testing.T) {
	t.Parallel()
	run := runSelfCheckCycle(t, config.StageEnforce, false)
	if run.err != nil {
		t.Fatalf("RunCycle: %v", run.err)
	}
	if run.builder.calls != 1 || len(run.builder.delivered) != 1 || !strings.Contains(run.builder.delivered[0], selfCheckCoverFinding) {
		t.Fatalf("build dispatches %d, deliveries %q; want one dispatch with one in-session delivery of the finding", run.builder.calls, run.builder.delivered)
	}
	if rec := readSelfCheckFindings(t, run.builder.workspace); rec.Decision != selfCheckDelivered || rec.Source != selfCheckSourceBridge {
		t.Errorf("findings record = %+v", rec)
	}
}

func TestBuilderSelfCheck_EnforceRecordsAFailedPasteAndKeepsTheCorrectionOfToday(t *testing.T) {
	t.Parallel()
	run := runSelfCheckCycle(t, config.StageEnforce, true)
	if run.err != nil {
		t.Fatalf("RunCycle: %v", run.err)
	}
	if run.builder.calls != 2 {
		t.Fatalf("build dispatches %d; a failed paste keeps the correction dispatch of today", run.builder.calls)
	}
	if rec := readSelfCheckFindings(t, run.builder.workspace); rec.Decision != selfCheckDeliveryFailed || rec.Error != "paste refused" {
		t.Errorf("findings record = %+v", rec)
	}
}

func TestBuilderSelfCheck_ShadowRunsNoFloorInTheWaitAndRecordsTheGateFindings(t *testing.T) {
	t.Parallel()
	run := runSelfCheckCycle(t, config.StageShadow, false)
	if run.err != nil {
		t.Fatalf("RunCycle: %v", run.err)
	}
	if run.floorInWait != 0 || run.floorAtGate == 0 {
		t.Fatalf("floor runs: %d in the wait, %d at the gate; shadow must add no floor run to the wait", run.floorInWait, run.floorAtGate)
	}
	if run.builder.calls != 2 || len(run.builder.delivered) != 0 {
		t.Fatalf("build dispatches %d, deliveries %d; want the initial dispatch plus one correction and no delivery", run.builder.calls, len(run.builder.delivered))
	}
	rec := readSelfCheckFindings(t, run.builder.workspace)
	if rec.Decision != selfCheckWouldReturn || rec.Stage != "shadow" || rec.Source != selfCheckSourceGate || rec.Receipt != receiptAbsent ||
		len(rec.Findings) != 1 || rec.Findings[0] != selfCheckCoverFinding {
		t.Errorf("findings record = %+v", rec)
	}
}

func TestRecordShadowSelfCheck_OnlyAFloorRejectionOfABuildWithNoGreenReceiptIsRecorded(t *testing.T) {
	t.Parallel()
	floorReject := ReviewResult{Reason: floorReasonHead + "1 deterministic check failure(s)", Failures: []string{selfCheckCoverFinding}}
	record := func(phase Phase, stage config.Stage, rr ReviewResult, green bool) bool {
		in := ReviewInput{Cycle: 9, Workspace: t.TempDir(), Worktree: selfCheckWorktree(t)}
		if green {
			if err := WriteSelfCheckReceipt(context.Background(), in.Workspace, in.Worktree, true); err != nil {
				t.Fatal(err)
			}
		}
		o := NewOrchestrator(nil, nil, nil, WithWorkflowConfig(policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: stage}}))
		(&cycleRun{o: o, ctx: context.Background()}).recordShadowSelfCheck(phase, in, rr)
		_, err := os.Stat(filepath.Join(in.Workspace, BuilderSelfCheckFindingsFile))
		return err == nil
	}
	if !record(PhaseBuild, config.StageShadow, floorReject, false) {
		t.Error("a floor rejection at shadow with no receipt is recorded")
	}
	for name, recorded := range map[string]bool{
		"enforce":       record(PhaseBuild, config.StageEnforce, floorReject, false),
		"off":           record(PhaseBuild, config.StageOff, floorReject, false),
		"audit":         record(PhaseAudit, config.StageShadow, floorReject, false),
		"approved":      record(PhaseBuild, config.StageShadow, ReviewResult{Approve: true}, false),
		"not the floor": record(PhaseBuild, config.StageShadow, ReviewResult{Reason: "[missing_section] build-report.md", Failures: []string{"x"}}, false),
		"green receipt": record(PhaseBuild, config.StageShadow, floorReject, true),
	} {
		if recorded {
			t.Errorf("%s: no shadow record is due", name)
		}
	}
}
