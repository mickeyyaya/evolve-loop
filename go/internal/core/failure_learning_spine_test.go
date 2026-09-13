package core

// failure_learning_spine_test.go — ADR-0103 unit 03b, commit 1: the pins on
// recordFailureLearning's spine that were comments (or nothing) before the
// split. Every test here is green on the pre-split code and red against the
// named mutant recorded in docs/architecture/decomposition/03b-failure-learning-engine.md
// §6/§8. Fakes and temp dirs only; the stderr-capturing tests must not run in
// parallel (os.Stderr is process-global — captureStderr's contract).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// spineRequest is the 11-field DTO the seven by-name tests build literally,
// built here ONCE for the spine pins (audit failed, cycle 1034, the workspace
// and project root in dir).
func spineRequest(dir string, err error) failureLearningRequest {
	return failureLearningRequest{
		CycleRequest: CycleRequest{ProjectRoot: dir},
		Cycle:        1034,
		Failed:       PhaseAudit,
		Err:          err,
		State:        &State{},
		CycleState:   &CycleState{CycleID: 1034, WorkspacePath: dir},
		Result:       &CycleResult{},
		Timings:      &[]phaseTimingEntry{},
		Context:      map[string]string{},
		Env:          map[string]string{},
	}
}

// seedValidDisposition models a compliant retro agent: the real input
// artifact, the canonical digest the assembler will derive, and a disposition
// carrying that identity (the disposition_gate_test.go fixture).
func seedValidDisposition(t *testing.T, dir string) {
	t.Helper()
	writeJSON(t, dir, "audit-fail-reason.json", map[string]any{
		"schema_version": 1, "phase": "audit",
		"reasons": []string{"EGPS: red_count=1 (cycle ships only when red_count==0)"},
	})
	digest, err := AssembleFailureDigest(1034, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := validDisposition()
	d["fingerprint"] = digest.Fingerprint
	d["recurrence"] = digest.Recurrence
	writeJSON(t, dir, "disposition.json", d)
}

// seqStorage logs the ORDER of the two storage effects the spine performs.
type seqStorage struct {
	*fakeStorage
	seq []string
}

func (s *seqStorage) WriteState(ctx context.Context, st State) error {
	s.seq = append(s.seq, "WriteState")
	return s.fakeStorage.WriteState(ctx, st)
}

func (s *seqStorage) WriteCycleState(ctx context.Context, cs CycleState) error {
	s.seq = append(s.seq, "WriteCycleState")
	return s.fakeStorage.WriteCycleState(ctx, cs)
}

// retroProbeRunner is a retro runner that inspects the workspace inside Run.
type retroProbeRunner struct {
	inRun   func(req PhaseRequest)
	verdict string
	err     error
}

func (p *retroProbeRunner) Name() string { return string(PhaseRetro) }
func (p *retroProbeRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if p.inRun != nil {
		p.inRun(req)
	}
	if p.err != nil {
		return PhaseResponse{}, p.err
	}
	return PhaseResponse{Phase: string(PhaseRetro), Verdict: p.verdict, ArtifactsDir: req.Workspace}, nil
}

// Test 1 — the no-retro-runner arm is characterised: record + todo persisted,
// NO retro stamp, NO cycle-state write, NO retro ledger entry, NO floor
// (the arm writes "queued carryover todo only" today — operator question 7).
func TestRecordFailureLearning_NoRetroRunner_QueuesTodoAndPersistsWithoutRetroStamp(t *testing.T) {
	dir := t.TempDir()
	st := &fakeStorage{}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, map[Phase]PhaseRunner{})
	fl := spineRequest(dir, errors.New("audit floor red"))
	stderr := captureStderr(t, func() { o.recordFailureLearning(context.Background(), fl) })
	if len(fl.State.FailedAt) != 1 || !carryoverTodoExists(fl.State.CarryoverTodos, "cycle-1034-failed-audit") {
		t.Fatalf("the arm records the failure and queues the todo: %+v %+v", fl.State.FailedAt, fl.State.CarryoverTodos)
	}
	if len(st.state.FailedAt) != 1 || len(st.state.CarryoverTodos) != 1 {
		t.Fatalf("the arm PERSISTS both (writeFailureLearningState on the no-runner arm): %+v", st.state)
	}
	if fl.CycleState.Phase == string(PhaseRetro) || fl.CycleState.ActiveAgent == string(PhaseRetro) || st.writeCSCalls != 0 {
		t.Fatalf("no retro stamp and no cycle-state write before the runner lookup: phase=%q agent=%q writes=%d", fl.CycleState.Phase, fl.CycleState.ActiveAgent, st.writeCSCalls)
	}
	if len(led.entries) != 0 {
		t.Fatalf("no retro ledger entry without a retro: %+v", led.entries)
	}
	lessons, _ := filepath.Glob(filepath.Join(dir, ".evolve", "instincts", "lessons", "*.yaml"))
	if _, err := os.Stat(filepath.Join(dir, "retrospective-report.md")); err == nil || len(lessons) != 0 {
		t.Fatalf("the arm writes no floor today (characterised, not fixed): report err=%v lessons=%v", err, lessons)
	}
	if !strings.Contains(stderr, "no retro runner registered; queued carryover todo only") {
		t.Fatalf("the :211 line verbatim, got %q", stderr)
	}
}

// Test 2 — the persist (WriteState) is the LAST storage effect on every exit
// that persists, and the three silent exits never persist. The pre-retro
// cycle-state write fires BEFORE runner.Run, so the runner-error and
// non-canonical arms log it too (landing-critic blocker 1).
func TestRecordFailureLearning_PersistIsTheLastStorageEffect(t *testing.T) {
	cases := []struct {
		name string
		prep func(t *testing.T, dir string, runners map[Phase]PhaseRunner)
		err  func() error
		ctx  func() context.Context
		want []string
	}{
		{"completed", func(t *testing.T, dir string, _ map[Phase]PhaseRunner) { seedValidDisposition(t, dir) },
			func() error { return errors.New("audit floor red") }, context.Background,
			[]string{"WriteCycleState", "WriteCycleState", "WriteState"}},
		{"doc-missing", func(*testing.T, string, map[Phase]PhaseRunner) {},
			func() error { return fmt.Errorf("audit: load agent: %w", ErrAgentDocMissing) }, context.Background,
			[]string{"WriteState"}},
		{"no-runner", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) { delete(r, PhaseRetro) },
			func() error { return errors.New("audit floor red") }, context.Background,
			[]string{"WriteState"}},
		{"retro-error", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) {
			r[PhaseRetro] = &retroProbeRunner{err: errors.New("retro boom")}
		}, func() error { return errors.New("audit floor red") }, context.Background,
			[]string{"WriteCycleState", "WriteState"}},
		{"non-canonical", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) {
			r[PhaseRetro] = &retroProbeRunner{verdict: "MAYBE"}
		}, func() error { return errors.New("audit floor red") }, context.Background,
			[]string{"WriteCycleState", "WriteState"}},
		{"nil-gate", func(*testing.T, string, map[Phase]PhaseRunner) {}, func() error { return nil }, context.Background, nil},
		{"canceled", func(*testing.T, string, map[Phase]PhaseRunner) {}, func() error { return errors.New("x") },
			func() context.Context { c, cancel := context.WithCancel(context.Background()); cancel(); return c }, nil},
		{"quota", func(*testing.T, string, map[Phase]PhaseRunner) {},
			func() error { return fmt.Errorf("phase audit: %w", ErrAllFamiliesExhausted) }, context.Background, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runners := buildRunners(nil)
			tc.prep(t, dir, runners)
			st := &seqStorage{fakeStorage: &fakeStorage{}}
			o := NewOrchestrator(st, &fakeLedger{}, runners)
			captureStderr(t, func() { o.recordFailureLearning(tc.ctx(), spineRequest(dir, tc.err())) })
			if !reflect.DeepEqual(st.seq, tc.want) {
				t.Fatalf("storage effects on the %s path = %v, want %v (the persist is LAST; silent exits never persist)", tc.name, st.seq, tc.want)
			}
		})
	}
}

// Test 3 — invariant B's engine half: the retro outcome is recorded AFTER the
// disposition gate set RetroDecision and BEFORE the persist; the timing log
// stays chronological [failed, retro]; the retro entry carries attempts 1 and
// the stamped PhaseStartedAt.
func TestRecordFailureLearning_RetroOutcomeIsRecordedAfterTheGateAndBeforeThePersist(t *testing.T) {
	dir := t.TempDir()
	seedValidDisposition(t, dir)
	c, _ := recordingCenter()
	st := &seqStorage{fakeStorage: &fakeStorage{}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	fl := spineRequest(dir, errors.New("audit floor red"))
	*fl.Timings = append(*fl.Timings, phaseTimingEntry{Phase: string(PhaseAudit), Verdict: VerdictFAIL})
	var decisionAtOutcome string
	var writesAtOutcome []string
	c.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindPhaseOutcome && e.Phase == string(PhaseRetro) {
			decisionAtOutcome = fl.Result.RetroDecision
			writesAtOutcome = append([]string(nil), st.seq...)
		}
	})
	o.recordFailureLearning(context.Background(), fl)
	if decisionAtOutcome != "failure-learning: queued cycle-1034-failed-audit" {
		t.Fatalf("RetroDecision must be set by the gate BEFORE the retro outcome records: %q", decisionAtOutcome)
	}
	if !reflect.DeepEqual(writesAtOutcome, []string{"WriteCycleState", "WriteCycleState"}) {
		t.Fatalf("the persist must come AFTER the retro outcome; storage at the outcome = %v", writesAtOutcome)
	}
	tm := *fl.Timings
	if len(tm) != 2 || tm[0].Phase != string(PhaseAudit) || tm[1].Phase != string(PhaseRetro) {
		t.Fatalf("timings [failed, retro], got %+v", tm)
	}
	if tm[1].AttemptCount != 1 || tm[1].AbortReason != "" || tm[1].StartedAt != fl.CycleState.PhaseStartedAt || tm[1].StartedAt == "" {
		t.Fatalf("the retro entry: attempts 1, no abort reason, startedAt == the stamped PhaseStartedAt: %+v vs %q", tm[1], fl.CycleState.PhaseStartedAt)
	}
}

// Test 4 — invariant B's caller half at the loud-abort site: the failed
// phase's outcome is recorded BEFORE the inline retro (timing log [scout FAIL, retro]).
func TestRunCycle_FailedPhaseOutcomeIsRecordedBeforeTheInlineRetro(t *testing.T) {
	st := &fakeStorage{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(1), failUntil: 99}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}); err == nil {
		t.Fatal("RunCycle: want the scout failure")
	}
	entries, err := phasetiming.Read(st.cycleState.WorkspacePath)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range entries {
		order = append(order, e.Phase+":"+e.Verdict)
	}
	if len(order) < 2 || order[len(order)-2] != "scout:FAIL" || order[len(order)-1] != "retro:PASS" {
		t.Fatalf("the abort site records the failed phase THEN the retro: %v", order)
	}
}

// Test 5 — the optional-skip site's order is pinned AS-IS: failure learning
// (its deterministic floor and persist) runs BEFORE recordPhaseSkip files the
// skip's ledger entry — the documented exception to invariant B (the skip's
// phase outcome is recorded by the dispatch loop after both), unit 05's to change.
func TestOptionalInfraSkip_RecordsTheInlineRetroBeforeTheSkipOutcome(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{}
	runners := buildRunners(nil)
	runners[Phase("amplify-tests")] = &fakeRunner{name: "amplify-tests",
		failErr: fmt.Errorf("amplify-tests: load agent: %w", ErrAgentDocMissing), failUntil: 99}
	led := &probeLedger{}
	lessonsAtSkip, persistedAtSkip := -1, -1
	led.onAppend = func(e LedgerEntry) {
		if e.Kind == "optional_missing_persona_skip" {
			lessons, _ := filepath.Glob(filepath.Join(root, ".evolve", "instincts", "lessons", "*.yaml"))
			lessonsAtSkip, persistedAtSkip = len(lessons), len(st.state.FailedAt)
		}
	}
	o := optionalPhaseOrchestrator(t, st, led, runners)
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("the optional skip must not abort: %v", err)
	}
	if lessonsAtSkip != 1 || persistedAtSkip != 1 {
		t.Fatalf("when the skip entry is filed the learning had already written its lesson and persisted its record (site order [learn, skip]); lessons=%d persisted=%d", lessonsAtSkip, persistedAtSkip)
	}
}

// probeLedger observes each Append (the fake ledger with a hook).
type probeLedger struct {
	fakeLedger
	onAppend func(LedgerEntry)
}

func (p *probeLedger) Append(ctx context.Context, e LedgerEntry) error {
	if p.onAppend != nil {
		p.onAppend(e)
	}
	return p.fakeLedger.Append(ctx, e)
}

// optionalPhaseOrchestrator is the missing-persona fixture (agent_doc_missing_test.go):
// an optional "amplify-tests" phase after build, dynamic-LLM routing with a fixed plan.
func optionalPhaseOrchestrator(t *testing.T, st Storage, led Ledger, runners map[Phase]PhaseRunner, extra ...Option) *Orchestrator {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "amplify-tests", Optional: true, After: "build"}})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build", "amplify-tests", "audit", "ship"}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true}, {Phase: "build", Run: true},
		{Phase: "amplify-tests", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	opts := append([]Option{WithRouting(cfg, router.StaticPreset{}), WithCatalog(cat), WithPlanner(&fixedPlanner{plan: plan})}, extra...)
	return NewOrchestrator(st, led, runners, opts...)
}

// Test 6 — invariant C's unobserved half: the failure digest exists on disk
// when the retro runner STARTS (ensureFailureDigest before runner.Run).
func TestRecordFailureLearning_DigestExistsWhenTheRetroRunnerStarts(t *testing.T) {
	dir := t.TempDir()
	runners := buildRunners(nil)
	present := false
	runners[PhaseRetro] = &retroProbeRunner{verdict: VerdictPASS, inRun: func(req PhaseRequest) {
		_, err := os.Stat(filepath.Join(req.Workspace, "failure-digest.json"))
		present = err == nil
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	o.recordFailureLearning(context.Background(), spineRequest(dir, errors.New("audit floor red")))
	if !present {
		t.Fatal("failure-digest.json must exist BEFORE the retro agent runs (it is the identity the disposition gate cross-checks)")
	}
}

// Test 7 — the doc-missing arm writes the digest (the assembler overwrites a
// foreign one) and lands the recurrence closure; the runner-error and
// non-canonical tails NEVER re-run the assembler (a deleted digest stays absent).
func TestRecordFailureLearning_DocMissingArmWritesTheDigestAndTheFallbackArmsNeverRewriteIt(t *testing.T) {
	t.Run("doc-missing arm assembles the digest and records the closure", func(t *testing.T) {
		dir := t.TempDir()
		writeMatchingDigest(t, dir)
		foreign, _ := os.ReadFile(filepath.Join(dir, "failure-digest.json"))
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
		captureStderr(t, func() {
			o.recordFailureLearning(context.Background(), spineRequest(dir, fmt.Errorf("audit: load agent: %w", ErrAgentDocMissing)))
		})
		after, err := os.ReadFile(filepath.Join(dir, "failure-digest.json"))
		if err != nil || string(after) == string(foreign) {
			t.Fatalf("the arm runs the assembler, which overwrites the foreign digest: err=%v same=%v", err, string(after) == string(foreign))
		}
		led, err := recurrence.Load(filepath.Join(dir, ".evolve", "recurrence-ledger.json"))
		if err != nil || led.Count("cycle-mid-execution-fail") == 0 {
			t.Fatalf("the floor's recurrence closure lands on the arm: err=%v count=%d", err, led.Count("cycle-mid-execution-fail"))
		}
	})
	for _, tail := range []struct {
		name   string
		runner PhaseRunner
	}{
		{"runner-error", &retroProbeRunner{err: errors.New("retro boom")}},
		{"non-canonical", &retroProbeRunner{verdict: "MAYBE"}},
	} {
		t.Run(tail.name+" tail never re-assembles the digest", func(t *testing.T) {
			dir := t.TempDir()
			runners := buildRunners(nil)
			pr := tail.runner.(*retroProbeRunner)
			pr.inRun = func(req PhaseRequest) { _ = os.Remove(filepath.Join(req.Workspace, "failure-digest.json")) }
			runners[PhaseRetro] = pr
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
			captureStderr(t, func() {
				o.recordFailureLearning(context.Background(), spineRequest(dir, errors.New("audit floor red")))
			})
			if _, err := os.Stat(filepath.Join(dir, "failure-digest.json")); err == nil {
				t.Fatal("the fallback tail must not run ensureFailureDigest a second time (the digest was written before Run)")
			}
		})
	}
}

// Test 8 — characterisation of the preserved quirks after a PASS inline retro:
// the phase stays stamped retro (Q1), cs.FinalVerdict is never stamped (Q6),
// Result.FinalVerdict is the retro's, RetroDecision names the queued todo.
func TestRecordFailureLearning_SuccessfulInlineRetroLeavesPhaseStampedRetro(t *testing.T) {
	dir := t.TempDir()
	seedValidDisposition(t, dir)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	fl := spineRequest(dir, errors.New("audit floor red"))
	o.recordFailureLearning(context.Background(), fl)
	cs := fl.CycleState
	if cs.Phase != string(PhaseRetro) || cs.ActiveAgent != string(PhaseRetro) {
		t.Fatalf("Q1 preserved: the phase stays stamped retro after a PASS inline retro: phase=%q agent=%q", cs.Phase, cs.ActiveAgent)
	}
	if n := len(cs.CompletedPhases); n == 0 || cs.CompletedPhases[n-1] != string(PhaseRetro) {
		t.Fatalf("CompletedPhases ends with retro: %v", cs.CompletedPhases)
	}
	if cs.FinalVerdict != "" || fl.Result.FinalVerdict != VerdictPASS {
		t.Fatalf("Q6 preserved: cs.FinalVerdict never stamped (%q); Result.FinalVerdict is the retro's (%q)", cs.FinalVerdict, fl.Result.FinalVerdict)
	}
	if fl.Result.RetroDecision != "failure-learning: queued cycle-1034-failed-audit" {
		t.Fatalf("RetroDecision: %q", fl.Result.RetroDecision)
	}
}

// Test 9 — the gate order nil → ctx → carrier → quota on the current spelling.
func TestRecordFailureLearning_GateOrderIsNilCtxCarrierQuota(t *testing.T) {
	exhausted := fmt.Errorf("phase ship: %w", ErrAllFamiliesExhausted)
	twice := fmt.Errorf("outer: %w", exhausted)
	canceled := func() context.Context { c, cancel := context.WithCancel(context.Background()); cancel(); return c }
	cases := []struct {
		name        string
		ctx         context.Context
		err         error
		nilResult   bool
		wantCarrier []string
		wantFailed  int
		wantStderr  string
	}{
		{"nil result: nothing recorded, no carrier", context.Background(), exhausted, true, nil, 0, ""},
		{"canceled ctx beats the carrier and the quota line", canceled(), twice, false, nil, 0, ""},
		{"quota: carrier written, nothing recorded, the DEFERRED line", context.Background(), twice, false, []string{twice.Error()}, 0, "skipping failure learning (DEFERRED, resumable)"},
		{"ordinary ship error: carrier written and recorded", context.Background(), errors.New("ship: push refused"), false, []string{"ship: push refused"}, 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
			fl := spineRequest(dir, tc.err)
			fl.Failed = PhaseShip
			if tc.nilResult {
				fl.Result = nil
			}
			stderr := captureStderr(t, func() { o.recordFailureLearning(tc.ctx, fl) })
			if !reflect.DeepEqual(fl.CycleState.ShipFailReasons, tc.wantCarrier) {
				t.Fatalf("ShipFailReasons = %q, want %q", fl.CycleState.ShipFailReasons, tc.wantCarrier)
			}
			if len(fl.State.FailedAt) != tc.wantFailed {
				t.Fatalf("FailedAt len = %d, want %d", len(fl.State.FailedAt), tc.wantFailed)
			}
			if tc.wantStderr == "" && strings.Contains(stderr, "DEFERRED") {
				t.Fatalf("no DEFERRED line on this path, got %q", stderr)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Fatalf("stderr %q lacks %q", stderr, tc.wantStderr)
			}
		})
	}
}

// Test 10 — the signal stream is byte-identical apart from the declared codes:
// the ordered {module/kind/code} sequence per path, captured on 97825125. The
// fallback paths emit NOTHING today (a fake ledger emits no ledger.appended; a
// PASS retro outcome is an INFO with no code), so a stray Emit or a dropped
// phase.outcome shows up as a diff. Commit 2 adds the provoked-fault row.
func TestRecordFailureLearning_StreamIsByteIdenticalApartFromTheDeclaredCodes(t *testing.T) {
	plain := func() error { return errors.New("audit floor red") }
	cases := []struct {
		name string
		prep func(t *testing.T, dir string, runners map[Phase]PhaseRunner)
		err  func() error
		want []string
	}{
		{"completed", func(t *testing.T, dir string, _ map[Phase]PhaseRunner) { seedValidDisposition(t, dir) }, plain, []string{"orchestrator/phase.outcome/"}},
		{"doc-missing", func(*testing.T, string, map[Phase]PhaseRunner) {}, func() error { return fmt.Errorf("audit: load agent: %w", ErrAgentDocMissing) }, nil},
		{"no-runner", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) { delete(r, PhaseRetro) }, plain, nil},
		{"retro-error", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) {
			r[PhaseRetro] = &retroProbeRunner{err: errors.New("retro boom")}
		}, plain, nil},
		{"non-canonical", func(_ *testing.T, _ string, r map[Phase]PhaseRunner) {
			r[PhaseRetro] = &retroProbeRunner{verdict: "MAYBE"}
		}, plain, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runners := buildRunners(nil)
			tc.prep(t, dir, runners)
			c, got := recordingCenter()
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithSignalCenter(c))
			captureStderr(t, func() { o.recordFailureLearning(context.Background(), spineRequest(dir, tc.err())) })
			c.Flush()
			if seq := streamSequence(*got); !reflect.DeepEqual(seq, tc.want) {
				t.Fatalf("stream on the %s path = %v, want %v", tc.name, seq, tc.want)
			}
		})
	}
}

func streamSequence(events []signalcenter.Event) []string {
	var seq []string
	for _, e := range events {
		seq = append(seq, fmt.Sprintf("%s/%s/%s", e.Module, e.Kind, e.Code))
	}
	return seq
}

// Test 11a — a failed retro ledger append raises no FAILURELEARNING_/ORCHESTRATOR_
// code (LEDGER_APPEND_FAILED at the adapter's one chokepoint is the signal);
// the retro outcome still records.
func TestRecordFailureLearning_RetroLedgerAppendFailure_RaisesOnlyTheLedgerCode(t *testing.T) {
	dir := t.TempDir()
	seedValidDisposition(t, dir)
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{failOnAppend: true}, buildRunners(nil), WithSignalCenter(c))
	fl := spineRequest(dir, errors.New("audit floor red"))
	captureStderr(t, func() { o.recordFailureLearning(context.Background(), fl) })
	c.Flush()
	for _, e := range *got {
		if strings.Contains(string(e.Code), "LEDGER") && (strings.HasPrefix(string(e.Code), "FAILURELEARNING_") || strings.HasPrefix(string(e.Code), "ORCHESTRATOR_")) {
			t.Fatalf("a failed append is the ledger adapter's LEDGER_APPEND_FAILED, never a second code: %+v", e)
		}
	}
	if tm := *fl.Timings; len(tm) != 1 || tm[0].Phase != string(PhaseRetro) {
		t.Fatalf("the retro outcome still records after a failed append: %+v", tm)
	}
}

// Test 36 — the gate is a PURE four-valued decision in the order nil → ctx →
// quota, and never touches the request (the ShipFailReasons carrier is the
// coordinator's, written between the gate and the quota return).
func TestFailureLearningGate_IsPureAndOrdered(t *testing.T) {
	exhausted := fmt.Errorf("outer: %w", fmt.Errorf("phase ship: %w", ErrAllFamiliesExhausted))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	base := func() failureLearningRequest {
		return failureLearningRequest{Cycle: 1, Failed: PhaseShip, Err: errors.New("x"), State: &State{}, CycleState: &CycleState{}, Result: &CycleResult{}, Timings: &[]phaseTimingEntry{}}
	}
	cases := []struct {
		name string
		ctx  context.Context
		edit func(*failureLearningRequest)
		want learningGate
	}{
		{"retro itself failed", context.Background(), func(fl *failureLearningRequest) { fl.Failed = PhaseRetro }, gateIncomplete},
		{"nil error", context.Background(), func(fl *failureLearningRequest) { fl.Err = nil }, gateIncomplete},
		{"nil State", context.Background(), func(fl *failureLearningRequest) { fl.State = nil }, gateIncomplete},
		{"nil CycleState", context.Background(), func(fl *failureLearningRequest) { fl.CycleState = nil }, gateIncomplete},
		{"nil Result", context.Background(), func(fl *failureLearningRequest) { fl.Result = nil }, gateIncomplete},
		{"nil Timings", context.Background(), func(fl *failureLearningRequest) { fl.Timings = nil }, gateIncomplete},
		{"canceled ctx beats the quota sentinel", canceled, func(fl *failureLearningRequest) { fl.Err = exhausted }, gateCanceled},
		{"wrapped quota sentinel", context.Background(), func(fl *failureLearningRequest) { fl.Err = exhausted }, gateQuotaDeferred},
		{"doc missing learns", context.Background(), func(fl *failureLearningRequest) { fl.Err = fmt.Errorf("x: %w", ErrAgentDocMissing) }, gateLearn},
		{"a plain error learns", context.Background(), func(*failureLearningRequest) {}, gateLearn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fl := base()
			tc.edit(&fl)
			if got := failureLearningGate(tc.ctx, fl); got != tc.want {
				t.Fatalf("gate = %v, want %v", got, tc.want)
			}
			if fl.CycleState != nil && fl.CycleState.ShipFailReasons != nil {
				t.Fatal("the gate is pure: the carrier is written by the coordinator, never here")
			}
		})
	}
}
