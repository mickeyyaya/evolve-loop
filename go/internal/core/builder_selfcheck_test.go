package core

import (
	"bytes"
	"context"
	"encoding/json"
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

	msg := check.beforeClose(context.Background())

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
	rec := readSelfCheckFindings(t, check.in.Workspace)
	if rec.Decision != selfCheckReturn || rec.Stage != "enforce" || len(rec.Findings) != 2 {
		t.Errorf("findings record = %+v", rec)
	}
	if len(*events) != 1 || (*events)[0].Code != CodeBuilderSelfCheck || (*events)[0].Fields["decision"] != selfCheckReturn || (*events)[0].Cycle != 2092 {
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

	if msg := check.beforeClose(context.Background()); msg != "" || floor.calls.Load() != 0 || len(*events) != 0 {
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

	if msg := check.beforeClose(context.Background()); !strings.Contains(msg, selfCheckCoverFinding) || floor.calls.Load() != 1 {
		t.Fatalf("a receipt for another tree: msg %q, floor calls %d; want the findings", msg, floor.calls.Load())
	}
}

func TestBuilderSelfCheck_ARedReceiptOrAnUnreadableOneIsNotGreen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	worktree, workspace := selfCheckWorktree(t), t.TempDir()
	if err := WriteSelfCheckReceipt(ctx, workspace, worktree, false); err != nil {
		t.Fatal(err)
	}
	if selfCheckReceiptGreen(ctx, workspace, worktree) {
		t.Error("a RED receipt counts as GREEN")
	}
	writeSelfCheckFile(t, workspace, SelfCheckReceiptFile, "{")
	if selfCheckReceiptGreen(ctx, workspace, worktree) {
		t.Error("an unparseable receipt counts as GREEN")
	}
	if selfCheckReceiptGreen(ctx, t.TempDir(), worktree) {
		t.Error("a missing receipt counts as GREEN")
	}
	if err := WriteSelfCheckReceipt(ctx, workspace, "", true); err == nil {
		t.Error("a receipt with no worktree must refuse, because it binds no tree")
	}
	if err := WriteSelfCheckReceipt(ctx, filepath.Join(workspace, SelfCheckReceiptFile, "sub"), worktree, true); err == nil {
		t.Error("an unwritable workspace must return the write error")
	}
}

func TestBuilderSelfCheck_ShadowRecordsTheFindingsAndDeliversNothing(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding}}
	check, events := newSelfCheck(t, config.StageShadow, floor)

	if msg := check.beforeClose(context.Background()); msg != "" {
		t.Fatalf("shadow delivered %q; the flow of today must run", msg)
	}
	rec := readSelfCheckFindings(t, check.in.Workspace)
	if rec.Decision != selfCheckWouldReturn || rec.Stage != "shadow" || len(rec.Findings) != 1 || rec.Findings[0] != selfCheckCoverFinding {
		t.Errorf("findings record = %+v", rec)
	}
	if len(*events) != 1 || (*events)[0].Fields["decision"] != selfCheckWouldReturn {
		t.Errorf("events = %+v", *events)
	}
}

func TestBuilderSelfCheck_NoFindingsRecordsAndDeliversNothing(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{}
	check, events := newSelfCheck(t, config.StageEnforce, floor)

	if msg := check.beforeClose(context.Background()); msg != "" || len(*events) != 0 {
		t.Fatalf("a clean floor: msg %q, events %d", msg, len(*events))
	}
	if _, err := os.Stat(filepath.Join(check.in.Workspace, BuilderSelfCheckFindingsFile)); !os.IsNotExist(err) {
		t.Errorf("a clean floor wrote a findings file: %v", err)
	}
}

func TestBuilderSelfCheck_AnUnwritableFindingsFileStillDelivers(t *testing.T) {
	t.Parallel()
	floor := &countingFloor{findings: []string{selfCheckCoverFinding}}
	check, events := newSelfCheck(t, config.StageEnforce, floor)
	writeSelfCheckFile(t, check.in.Workspace, "not-a-dir", "")
	check.in.Workspace = filepath.Join(check.in.Workspace, "not-a-dir")
	var warn bytes.Buffer
	check.errOut = &warn

	if msg := check.beforeClose(context.Background()); !strings.Contains(msg, selfCheckCoverFinding) || len(*events) != 1 {
		t.Fatalf("msg %q events %d; the signal and the delivery do not depend on the file", msg, len(*events))
	}
	if !strings.Contains(warn.String(), "[builder-selfcheck] WARN findings record not written") {
		t.Errorf("a failed findings write must warn on the orchestrator output: %q", warn.String())
	}
}

func TestBuilderSelfCheckBeforeClose_OnlyABuildWithAFloorAndAStageGetsTheCheck(t *testing.T) {
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
	if run(PhaseBuild, config.StageShadow, floor) == nil || run(PhaseBuild, config.StageEnforce, floor) == nil {
		t.Error("a build at shadow or enforce with a floor gets the check")
	}
	if run(PhaseBuild, config.StageOff, floor) != nil {
		t.Error("stage off gets no check")
	}
	if run(PhaseAudit, config.StageEnforce, floor) != nil {
		t.Error("a phase other than build gets no check")
	}
	bare := &cycleRun{o: &Orchestrator{workflowConfig: policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: config.StageEnforce}}}}
	if bare.builderSelfCheckBeforeClose(PhaseBuild) != nil {
		t.Error("an orchestrator with no reviewer has no handoff floor and gets no check")
	}
	if run(PhaseBuild, config.StageEnforce, noopReviewer{}) == nil {
		t.Error("any reviewer carries the mandatory explanation floor, so the build gets the check")
	}
}

type inSessionBuilder struct {
	*fakeRunner
	fixed     *atomic.Bool
	delivered []string
}

func (b *inSessionBuilder) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if req.BeforeClose != nil {
		if msg := req.BeforeClose(ctx); msg != "" {
			b.delivered = append(b.delivered, msg)
			b.fixed.Store(true)
		}
	}
	if req.CorrectionDirective != "" {
		b.fixed.Store(true)
	}
	return b.fakeRunner.Run(ctx, req)
}

func runSelfCheckCycle(t *testing.T, stage config.Stage) (*inSessionBuilder, error) {
	t.Helper()
	fixed := &atomic.Bool{}
	floor := BuildHandoffFloor{{Name: "fixture", Run: func(context.Context, ReviewInput) []string {
		if fixed.Load() {
			return nil
		}
		return []string{selfCheckCoverFinding}
	}}}
	runners := buildRunners(nil)
	builder := &inSessionBuilder{fakeRunner: runners[PhaseBuild].(*fakeRunner), fixed: fixed}
	runners[PhaseBuild] = builder
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithReviewer(floor),
		WithWorkflowConfig(policy.WorkflowConfig{BuilderSelfCheck: policy.BuilderSelfCheckConfig{Stage: stage}}))
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	return builder, err
}

func TestBuilderSelfCheck_EnforceFixesTheFloorInTheSameSessionWithNoNewDispatch(t *testing.T) {
	t.Parallel()
	builder, err := runSelfCheckCycle(t, config.StageEnforce)
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if builder.calls != 1 || len(builder.delivered) != 1 || !strings.Contains(builder.delivered[0], selfCheckCoverFinding) {
		t.Fatalf("build dispatches %d, deliveries %q; want one dispatch with one in-session delivery of the finding", builder.calls, builder.delivered)
	}
}

func TestBuilderSelfCheck_ShadowKeepsTheCorrectionDispatchOfToday(t *testing.T) {
	t.Parallel()
	builder, err := runSelfCheckCycle(t, config.StageShadow)
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if builder.calls != 2 || len(builder.delivered) != 0 {
		t.Fatalf("build dispatches %d, deliveries %d; want the initial dispatch plus one correction and no delivery", builder.calls, len(builder.delivered))
	}
}
