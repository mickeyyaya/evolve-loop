package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const staleCycle = 42

type staleExplanationFixture struct {
	root, worktree, workspace, base, runID string
}

func newStaleExplanationFixture(t *testing.T) staleExplanationFixture {
	t.Helper()
	f := staleExplanationFixture{root: t.TempDir(), worktree: initTempGitRepo(t), runID: "run-42"}
	f.workspace = filepath.Join(f.root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(f.workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app.yaml", "enabled: false\n")
	runGit(t, f.worktree, "add", "-A")
	runGit(t, f.worktree, "commit", "-q", "-m", "base")
	f.base = gitOut(t, f.worktree, "rev-parse", "HEAD")
	activation := f.binding()
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(f.binding()); err != nil {
		t.Fatal(err)
	}
	f.authorCitingTestFile(t)
	return f
}

func (f staleExplanationFixture) binding() explanationdocs.CycleBinding {
	return explanationdocs.CycleBinding{
		ProjectRoot: f.root, Worktree: f.worktree, Workspace: f.workspace, BaseSHA: f.base,
		Cycle: staleCycle, RunID: f.runID, ContractVersion: explanationdocs.CurrentContractVersion,
	}
}

func (f staleExplanationFixture) write(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(f.worktree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f staleExplanationFixture) authorCitingTestFile(t *testing.T) {
	t.Helper()
	document, err := explanationdocs.DocumentPath(staleCycle, f.runID)
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "config/app.yaml", "enabled: true\n")
	f.write(t, "config/app_test.go", "package config_test\n")
	f.write(t, document, "# Build Explanation — Cycle 42\n\n"+
		"## Build Binding\n- Cycle: 42\n- Base SHA: "+f.base+"\n\n"+
		"## Summary\nEnable the application through its existing configuration field.\n\n"+
		"## Rationale\nUsing the existing field is the smallest compatible behavior change and avoids a second configuration surface.\n\n"+
		"## Changed Areas\n- `config/app.yaml` — flips the existing runtime setting while preserving its schema.\n"+
		"- `config/app_test.go` — pins the enabled setting through the public field.\n\n"+
		"## Design Decisions\nThe existing YAML setting remains the only public control for this behavior.\n\n"+
		"## Verification\nThe targeted configuration tests exercise both enabled and disabled behavior.\n\n"+
		"## Compatibility\nThe schema and setting name remain unchanged.\n\n"+
		"## Limitations\nThis does not add per-user configuration.\n")
	report := "# Build Report\n\n## Explanation Documentation\n- Status: REQUIRED\n- Document: " + document + "\n"
	if err := os.WriteFile(filepath.Join(f.workspace, "build-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := explanationdocs.CheckBuild(context.Background(), f.binding()); len(failures) != 0 {
		t.Fatalf("CheckBuild before the post-Build writer: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), f.binding()); err != nil {
		t.Fatalf("SealResult: %v", err)
	}
}

func (f staleExplanationFixture) postBuildWriterDropsCitedTest(t *testing.T) {
	t.Helper()
	if err := os.Remove(filepath.Join(f.worktree, "config", "app_test.go")); err != nil {
		t.Fatal(err)
	}
}

func (f staleExplanationFixture) cycleState(attempts int) CycleState {
	return CycleState{
		CycleID: staleCycle, RunID: f.runID, WorkspacePath: f.workspace,
		ActiveWorktree: f.worktree, WorktreeBaseSHA: f.base,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		CompletedPhases:                 []string{string(PhaseScout), string(PhaseBuild), string(PhaseAudit)},
		AuditRepairAttempts:             attempts,
	}
}

const amplifier Phase = "test-amplification"

func amplifierCatalog(t *testing.T) Option {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: string(amplifier), Optional: true, WritesSource: true}})
	if err != nil {
		t.Fatal(err)
	}
	return WithCatalog(cat)
}

func eventsWithCode(events []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

func (f staleExplanationFixture) freshRun(attempts int, opts ...Option) *cycleRun {
	return &cycleRun{
		o:     NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...),
		ctx:   context.Background(),
		req:   CycleRequest{ProjectRoot: f.root},
		cycle: staleCycle,
		cs:    f.cycleState(attempts),
	}
}

func passed(writer Phase) *dispatchResult {
	return &dispatchResult{resp: PhaseResponse{Phase: string(writer), Verdict: VerdictPASS}}
}

func retroDispatches(cr *cycleRun) int {
	return cr.o.runners[PhaseRetro].(*fakeRunner).calls
}

func onlyReauthorSignal(t *testing.T, got []signalcenter.Event) signalcenter.Event {
	t.Helper()
	routed := eventsWithCode(got, CodeExplanationReauthorRouted)
	if len(routed) != 1 {
		t.Fatalf("want ONE %s signal, got %d: %+v", CodeExplanationReauthorRouted, len(routed), got)
	}
	e := routed[0]
	if e.Kind != signalcenter.KindPhaseOutcome || e.Severity != signalcenter.SeverityWarn || e.Cycle != staleCycle || e.Fields["next"] != string(PhaseBuild) {
		t.Errorf("the re-author signal is a WARN phase outcome naming the cycle and Build as the re-entry: %+v", e)
	}
	if !strings.Contains(e.Reason, "cited path config/app_test.go is not in the Build diff") {
		t.Errorf("the signal carries the validator's verdict: %q", e.Reason)
	}
	if e.Attempt != 0 {
		t.Errorf("a routing disposition is not a dispatch attempt; the spent attempt rides fields.attempt: Attempt=%d", e.Attempt)
	}
	if _, ok := signalcenter.IsRegistered(CodeExplanationReauthorRouted); !ok {
		t.Errorf("%s is not a registered code", CodeExplanationReauthorRouted)
	}
	return e
}

func TestApplyPostReviewGuards_AStaleCitationAfterTDDRoutesToBuildWithoutSpendingTheBudget(t *testing.T) {
	for _, attempts := range []int{1, 2} {
		t.Run(fmt.Sprintf("attempts=%d", attempts), func(t *testing.T) {
			f := newStaleExplanationFixture(t)
			f.postBuildWriterDropsCitedTest(t)
			c, got := recordingCenter()
			cr := f.freshRun(attempts, WithSignalCenter(c))

			action, err := cr.applyPostReviewGuards(PhaseTDD, passed(PhaseTDD))

			if err != nil || action != loopNext || cr.scheduledNext != PhaseBuild {
				t.Fatalf("every successor of tdd leads to Build, so a stale citation routes there and never aborts: action=%v next=%q err=%v", action, cr.scheduledNext, err)
			}
			if cr.cs.AuditRepairAttempts != attempts {
				t.Errorf("the Build that follows tdd anyway costs the audit no repair: attempts=%d, want %d", cr.cs.AuditRepairAttempts, attempts)
			}
			e := onlyReauthorSignal(t, *got)
			if e.Phase != string(PhaseTDD) || e.Fields["charged"] != "false" || e.Fields["attempt"] != "" || e.Fields["envelope"] != "" {
				t.Errorf("an uncharged round names its writer and carries no budget fields: %+v", e)
			}
		})
	}
}

func TestApplyPostReviewGuards_AStaleCitationAfterABudgetedWriterSpendsOneRepairAttempt(t *testing.T) {
	for name, tc := range map[string]struct {
		attempts     int
		opts         []Option
		wantEnvelope string
	}{
		"the compiled policy":       {attempts: 1, wantEnvelope: "policy code-audit-fail ⇒ retry-with-fix (address-audit-findings), attempt 2/2"},
		"an injected policy budget": {attempts: 2, opts: []Option{WithFailurePolicy(auditRetries(3))}, wantEnvelope: "policy code-audit-fail ⇒ retry-with-fix (address-audit-findings), attempt 3/3"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newStaleExplanationFixture(t)
			f.postBuildWriterDropsCitedTest(t)
			c, got := recordingCenter()
			cr := f.freshRun(tc.attempts, append([]Option{WithSignalCenter(c), amplifierCatalog(t)}, tc.opts...)...)

			action, err := cr.applyPostReviewGuards(amplifier, passed(amplifier))

			if err != nil || action != loopNext || cr.scheduledNext != PhaseBuild {
				t.Fatalf("a budgeted writer's stale citation routes to Build within budget: action=%v next=%q err=%v", action, cr.scheduledNext, err)
			}
			if cr.cs.AuditRepairAttempts != tc.attempts+1 {
				t.Errorf("the round spends one code-audit-fail attempt: attempts=%d, want %d", cr.cs.AuditRepairAttempts, tc.attempts+1)
			}
			e := onlyReauthorSignal(t, *got)
			if e.Phase != string(amplifier) || e.Fields["charged"] != "true" || e.Fields["attempt"] != fmt.Sprint(tc.attempts+1) || e.Fields["envelope"] != tc.wantEnvelope {
				t.Errorf("a charged round names the attempt spent and the budget's own reason %q: %+v", tc.wantEnvelope, e.Fields)
			}
		})
	}
}

func auditRetries(max int) policy.SystemFailurePolicy {
	fp := policy.DefaultSystemFailurePolicy()
	row := fp.Categories[policy.CategoryCodeAuditFail]
	row.MaxRetries = max
	fp.Categories[policy.CategoryCodeAuditFail] = row
	return fp
}

func TestApplyPostReviewGuards_ABudgetedWriterPastItsBudgetEndsTheCycleNotTheBatch(t *testing.T) {
	f := newStaleExplanationFixture(t)
	f.postBuildWriterDropsCitedTest(t)
	c, got := recordingCenter()
	cr := f.freshRun(2, WithSignalCenter(c), amplifierCatalog(t))

	action, err := cr.applyPostReviewGuards(amplifier, passed(amplifier))

	var cycleLevel *ErrCycleLevelFailure
	if err == nil || action != loopAbort || !errors.As(err, &cycleLevel) || cycleLevel.Phase != string(amplifier) {
		t.Fatalf("a spent budget aborts as a cycle-level failure naming the writer, so the batch continues: action=%v failure=%+v err=%v", action, cycleLevel, err)
	}
	if !errors.Is(err, explanationdocs.ErrContent) || !strings.Contains(err.Error(), "retry budget spent for code-audit-fail (2/2)") {
		t.Errorf("the abort names the content failure and the spent budget: %v", err)
	}
	if cr.scheduledNext != "" || cr.cs.AuditRepairAttempts != 2 || len(eventsWithCode(*got, CodeExplanationReauthorRouted)) != 0 {
		t.Errorf("nothing is scheduled, spent or signalled past the budget: next=%q attempts=%d", cr.scheduledNext, cr.cs.AuditRepairAttempts)
	}
	if n := retroDispatches(cr); n != 1 {
		t.Errorf("the abort feeds failure learning once: retro dispatches=%d", n)
	}
}

func TestApplyPostReviewGuards_BindingMismatchStillAbortsAsBatchFatal(t *testing.T) {
	f := newStaleExplanationFixture(t)
	f.postBuildWriterDropsCitedTest(t)
	c, got := recordingCenter()
	cr := f.freshRun(0, WithSignalCenter(c))
	cr.cs.ActiveWorktree = t.TempDir()

	action, err := cr.applyPostReviewGuards(PhaseTDD, passed(PhaseTDD))

	var cycleLevel *ErrCycleLevelFailure
	if err == nil || action != loopAbort || !strings.Contains(err.Error(), "refresh Build explanation after tdd") {
		t.Fatalf("a binding-level failure must abort loudly: action=%v err=%v", action, err)
	}
	if errors.Is(err, explanationdocs.ErrContent) || errors.As(err, &cycleLevel) {
		t.Errorf("a binding mismatch is neither explanation content nor a cycle-level failure: %v", err)
	}
	if cr.scheduledNext != "" || cr.cs.AuditRepairAttempts != 0 || len(eventsWithCode(*got, CodeExplanationReauthorRouted)) != 0 {
		t.Errorf("a binding abort spends nothing and schedules nothing: next=%q attempts=%d", cr.scheduledNext, cr.cs.AuditRepairAttempts)
	}
	if n := retroDispatches(cr); n != 1 {
		t.Errorf("the abort feeds failure learning once: retro dispatches=%d", n)
	}
}

type postBuildWriter struct {
	phase  Phase
	effect func() error
}

func (r postBuildWriter) Name() string { return string(r.phase) }

func (r postBuildWriter) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	if err := r.effect(); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: string(r.phase), Verdict: VerdictPASS}, nil
}

func (f staleExplanationFixture) dropsCitedTest() error {
	return os.Remove(filepath.Join(f.worktree, "config", "app_test.go"))
}

func (f staleExplanationFixture) dropsBuildReport() error {
	return os.Remove(filepath.Join(f.workspace, "build-report.md"))
}

var errBuildReached = errors.New("build dispatched after the refresh")

type stopAtBuild struct{ calls int }

func (r *stopAtBuild) Name() string { return string(PhaseBuild) }

func (r *stopAtBuild) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	r.calls++
	return PhaseResponse{}, errBuildReached
}

type resumeRig struct {
	storage *fakeStorage
	builder *stopAtBuild
	planner *fakeRunner
	o       *Orchestrator
	got     *[]signalcenter.Event
}

func (f staleExplanationFixture) resumeRig(attempts int, writer postBuildWriter, opts ...Option) resumeRig {
	rig := resumeRig{storage: &fakeStorage{cycleState: f.cycleState(attempts)}, builder: &stopAtBuild{}, planner: &fakeRunner{name: string(PhaseBuildPlanner)}}
	runners := buildRunners(nil)
	runners[writer.phase] = writer
	runners[PhaseBuildPlanner] = rig.planner
	runners[PhaseBuild] = rig.builder
	c, got := recordingCenter()
	rig.got = got
	rig.o = NewOrchestrator(rig.storage, &fakeLedger{}, runners, append([]Option{WithSignalCenter(c)}, opts...)...)
	return rig
}

func (rig resumeRig) resumeAt(f staleExplanationFixture, writer Phase) (CycleResult, error) {
	return rig.o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: f.root}, &ResumePoint{Phase: string(writer), CycleID: staleCycle})
}

func TestRunCycleFromPhase_AStaleCitationAfterAResumedTDDRoutesToBuildWithoutSpending(t *testing.T) {
	f := newStaleExplanationFixture(t)
	rig := f.resumeRig(2, postBuildWriter{phase: PhaseTDD, effect: f.dropsCitedTest})

	_, err := rig.resumeAt(f, PhaseTDD)

	if strings.Contains(errorText(err), "resume refresh Build explanation") {
		t.Fatalf("the resumed refresh aborted on a stale citation instead of re-authoring: %v", err)
	}
	if !errors.Is(err, errBuildReached) || rig.builder.calls != 1 || rig.planner.calls != 0 {
		t.Fatalf("the resume must dispatch Build next, before tdd's natural successor: builder calls=%d build-planner calls=%d err=%v", rig.builder.calls, rig.planner.calls, err)
	}
	if spent := maxPersistedRepairAttempts(rig.storage); spent != 2 {
		t.Errorf("the resumed tdd round spends nothing: max attempts=%d, want 2", spent)
	}
	if e := onlyReauthorSignal(t, *rig.got); e.Fields["charged"] != "false" {
		t.Errorf("the resumed round is uncharged: %+v", e.Fields)
	}
}

func TestRunCycleFromPhase_AResumedRefreshAbortStopsBeforeBuild(t *testing.T) {
	for name, tc := range map[string]struct {
		writer         func(f staleExplanationFixture) postBuildWriter
		opts           func(t *testing.T) []Option
		wantContent    bool
		wantCycleLevel bool
	}{
		"a budgeted writer past its budget": {
			writer: func(f staleExplanationFixture) postBuildWriter {
				return postBuildWriter{phase: amplifier, effect: f.dropsCitedTest}
			},
			opts:        func(t *testing.T) []Option { return []Option{amplifierCatalog(t)} },
			wantContent: true, wantCycleLevel: true,
		},
		"a writer that leaves the Build report unreadable": {
			writer: func(f staleExplanationFixture) postBuildWriter {
				return postBuildWriter{phase: PhaseTDD, effect: f.dropsBuildReport}
			},
			opts: func(*testing.T) []Option { return nil },
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newStaleExplanationFixture(t)
			writer := tc.writer(f)
			rig := f.resumeRig(2, writer, tc.opts(t)...)

			result, err := rig.resumeAt(f, writer.phase)

			var cycleLevel *ErrCycleLevelFailure
			if !strings.Contains(errorText(err), "resume refresh Build explanation after "+string(writer.phase)) || rig.builder.calls != 0 {
				t.Fatalf("the resume root aborts at the refresh, before Build: builder calls=%d err=%v", rig.builder.calls, err)
			}
			if errors.Is(err, explanationdocs.ErrContent) != tc.wantContent || errors.As(err, &cycleLevel) != tc.wantCycleLevel {
				t.Errorf("content=%v cycle-level=%v, want %v/%v: %v", errors.Is(err, explanationdocs.ErrContent), errors.As(err, &cycleLevel), tc.wantContent, tc.wantCycleLevel, err)
			}
			if result.FinalVerdict != VerdictFAIL || len(eventsWithCode(*rig.got, CodeExplanationReauthorRouted)) != 0 {
				t.Errorf("the aborted resume seals FAIL and takes no rung: verdict=%q", result.FinalVerdict)
			}
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func maxPersistedRepairAttempts(s *fakeStorage) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	spent := s.cycleState.AuditRepairAttempts
	for _, cs := range s.cycleStateLog {
		spent = max(spent, cs.AuditRepairAttempts)
	}
	return spent
}

func (f staleExplanationFixture) changesMaterialAfterBuild() error {
	return os.WriteFile(filepath.Join(f.worktree, "config", "app.yaml"), []byte("enabled: maybe\n"), 0o644)
}

func TestApplyPostReviewGuards_AMaterialScopeChangeAfterAWriterRoutesToBuildWithoutCharge(t *testing.T) {
	for _, writer := range []Phase{PhaseTDD, amplifier} {
		t.Run(string(writer), func(t *testing.T) {
			f := newStaleExplanationFixture(t)
			if err := f.changesMaterialAfterBuild(); err != nil {
				t.Fatal(err)
			}
			c, got := recordingCenter()
			cr := f.freshRun(2, WithSignalCenter(c), amplifierCatalog(t))

			action, err := cr.applyPostReviewGuards(writer, passed(writer))

			if err != nil || action != loopNext || cr.scheduledNext != PhaseBuild {
				t.Fatalf("a material change after Build routes back to Build: action=%v next=%q err=%v", action, cr.scheduledNext, err)
			}
			if cr.cs.AuditRepairAttempts != 2 || len(eventsWithCode(*got, CodeExplanationReauthorRouted)) != 0 {
				t.Errorf("the material route spends nothing and is not a re-author rung: attempts=%d", cr.cs.AuditRepairAttempts)
			}
		})
	}
}

func TestRunCycleFromPhase_AResumedMaterialScopeChangeRoutesToBuild(t *testing.T) {
	f := newStaleExplanationFixture(t)
	rig := f.resumeRig(2, postBuildWriter{phase: PhaseTDD, effect: f.changesMaterialAfterBuild})

	_, err := rig.resumeAt(f, PhaseTDD)

	if !errors.Is(err, errBuildReached) || rig.builder.calls != 1 || rig.planner.calls != 0 {
		t.Fatalf("a resumed material change routes straight to Build, before tdd's natural successor: builder=%d planner=%d err=%v", rig.builder.calls, rig.planner.calls, err)
	}
	if spent := maxPersistedRepairAttempts(rig.storage); spent != 2 || len(eventsWithCode(*rig.got, CodeExplanationReauthorRouted)) != 0 {
		t.Errorf("the resumed material route spends nothing and takes no re-author rung: max attempts=%d", spent)
	}
}
