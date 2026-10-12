package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestEvaluateBatch_ANonEvaluatePhaseEndsTheRunButNotBeforeItStarts(t *testing.T) {
	t.Parallel()
	arch := map[string]string{"scout": "plan", "build": "build", "e1": "evaluate", "e2": "evaluate", "e3": "evaluate", "e4": "evaluate", "memo": "control", "audit": "evaluate"}
	of := func(p string) string { return arch[p] }
	cases := []struct {
		name string
		plan []string
		want []string
	}{
		{"a control phase after two evaluates ends the run", []string{"build", "e1", "e2", "memo", "e3", "e4", "audit"}, []string{"e1", "e2"}},
		{"a control phase before the run is skipped over", []string{"build", "memo", "e1", "e2", "audit"}, []string{"e1", "e2"}},
		{"a control phase after one evaluate leaves nothing to batch", []string{"build", "e1", "memo", "e2", "e3", "audit"}, nil},
		{"the run reaches the end of the plan without audit", []string{"scout", "build", "e1", "e2", "e3"}, []string{"e1", "e2", "e3"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateBatch(tc.plan, of); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("evaluateBatch(%v) = %v, want %v", tc.plan, got, tc.want)
			}
		})
	}
}

func FuzzEvaluateBatch_IsAContiguousPostBuildEvaluateRun(f *testing.F) {
	f.Add("build,e1,e2,audit")
	f.Add("e1,build,x,e1,e2,x,e3")
	f.Add(",,build,,")
	f.Add("audit,build,e9,e8")
	f.Fuzz(func(t *testing.T, csv string) {
		plan := strings.Split(csv, ",")
		of := func(p string) string {
			if strings.HasPrefix(p, "e") {
				return "evaluate"
			}
			return "other"
		}
		got := evaluateBatch(plan, of)
		if got == nil {
			return
		}
		if len(got) < 2 {
			t.Fatalf("a batch of %d phases: %v", len(got), got)
		}
		start := -1
		for i, p := range plan {
			if p == "build" {
				start = i
				break
			}
		}
		if start < 0 {
			t.Fatalf("batched %v with no build in %v", got, plan)
		}
		joined := "," + strings.Join(plan[start+1:], ",") + ","
		if !strings.Contains(joined, ","+strings.Join(got, ",")+",") {
			t.Fatalf("batch %v is not contiguous after build in %v", got, plan)
		}
		for _, p := range got {
			if of(p) != "evaluate" || p == "audit" {
				t.Fatalf("non-evaluate %q batched", p)
			}
		}
	})
}

func TestPhaseRequestFor_AnEnforcedPhaseIOSealsTheTypedInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stage      config.Stage
		wantActive bool
		wantShadow bool
	}{
		{config.StageOff, false, false},
		{config.StageShadow, false, true},
		{config.StageEnforce, true, true},
	}
	for _, tc := range cases {
		cr := newBatchCycleRun(t, buildRunners(nil), 1)
		cr.o.cfg.PhaseIO = tc.stage
		cr.cycle = 23

		req := cr.phaseRequestFor(Phase("tester"))

		if req.Input.Active() != tc.wantActive {
			t.Fatalf("stage %s: Input.Active = %v, want %v", tc.stage, req.Input.Active(), tc.wantActive)
		}
		if tc.wantActive && (req.Input.Phase != "tester" || req.Input.Cycle != 23) {
			t.Fatalf("sealed input = phase %q cycle %d, want tester/23", req.Input.Phase, req.Input.Cycle)
		}
		_, statErr := os.Stat(filepath.Join(cr.cs.WorkspacePath, "phaseio-shadow-tester.json"))
		if (statErr == nil) != tc.wantShadow {
			t.Fatalf("stage %s: shadow artifact present = %v, want %v", tc.stage, statErr == nil, tc.wantShadow)
		}
	}
}

func TestDispatchEvaluateBatch_ANonPositiveConcurrencyRunsTheBatchSerially(t *testing.T) {
	t.Parallel()
	for _, conc := range []int{0, -3} {
		var active, maxActive int32
		runners := buildRunners(nil)
		runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &active, maxActive: &maxActive}
		runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &active, maxActive: &maxActive}
		cr := newBatchCycleRun(t, runners, conc)

		act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})

		if err != nil || act != loopNext {
			t.Fatalf("concurrency %d: act=%v err=%v", conc, act, err)
		}
		if maxActive != 1 || !batchHas(cr, "tester") || !batchHas(cr, "evaluator") {
			t.Fatalf("concurrency %d: peak %d, phases %v; want both run one at a time", conc, maxActive, cr.result.PhasesRun)
		}
	}
}

func TestDispatchEvaluateBatch_ACycleStateWriteFailureAbortsAfterRecordingTheMembers(t *testing.T) {
	t.Parallel()
	var a, m int32
	runners := buildRunners(nil)
	runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &a, maxActive: &m}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &a, maxActive: &m}
	cr := newBatchCycleRun(t, runners, 2)
	cr.o.storage = &fakeStorage{failOnWriteCS: true}

	act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})

	if act != loopAbort || err == nil || !strings.Contains(err.Error(), "write cycle-state post-evaluate-batch: write CS forced fail") {
		t.Fatalf("act=%v err=%v, want an abort naming the cycle-state write", act, err)
	}
	if !reflect.DeepEqual(cr.cs.CompletedPhases, []string{"tester", "evaluator"}) || cr.lastVerdict != "" {
		t.Fatalf("completed %v lastVerdict %q: members recorded, verdict not adopted", cr.cs.CompletedPhases, cr.lastVerdict)
	}
}

func TestRecordBatchMember_ALedgerFailureWarnsAndTheBatchStillAdvances(t *testing.T) {
	var a, m int32
	runners := buildRunners(nil)
	runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &a, maxActive: &m, verdict: VerdictWARN}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &a, maxActive: &m}
	cr := newBatchCycleRun(t, runners, 2)
	ledger := &fakeLedger{failOnAppend: true}
	cr.o.ledger = ledger

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"}) })

	if err != nil || act != loopNext || cr.lastVerdict != VerdictWARN {
		t.Fatalf("act=%v err=%v verdict=%s; a lost ledger row must not fail the batch", act, err, cr.lastVerdict)
	}
	for _, p := range []string{"tester", "evaluator"} {
		if !strings.Contains(log, "WARN evaluate-batch ledger append "+p+": forced ledger append fail") {
			t.Errorf("stderr lacks the %s ledger warning:\n%s", p, log)
		}
	}
}

func TestRetryPhaseRunner_ABackfilledArtifactTimeoutReturnsTheWarnResponse(t *testing.T) {
	runner := &alwaysFailRunner{name: "build", err: ErrArtifactTimeout}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, map[Phase]PhaseRunner{PhaseBuild: runner})
	o.retryConfig.PhaseMaxAttempts = 2
	o.retryConfig.RetryBackoffBaseS = 0
	cr := retryParityCycleRun(o, t)
	cr.workflowConfig.BackfillEnabled = true
	report := "# Build Report\n\n" + strings.Repeat("implemented the change and verified it.\n", 8)
	writeFile(t, filepath.Join(cr.cs.WorkspacePath, "build-stdout.clean.txt"), "noise\n"+report)

	var resp PhaseResponse
	var attempts int
	var err error
	captureStderr(t, func() {
		resp, attempts, err = cr.retryPhaseRunner(PhaseBuild, PhaseRequest{}, cr.mainDispatchRetryOpts())
	})

	if err != nil || resp.Verdict != VerdictWARN || attempts != 2 || runner.n != 2 {
		t.Fatalf("resp=%+v attempts=%d runs=%d err=%v; want WARN after both attempts", resp, attempts, runner.n, err)
	}
	body, rerr := os.ReadFile(backfillArtifactPath(cr.cs.WorkspacePath, string(PhaseBuild)))
	if rerr != nil || !strings.HasPrefix(string(body), "# Build Report") {
		t.Fatalf("backfilled artifact = %q, %v", body, rerr)
	}
}

func TestRetryPhaseRunner_NoBackfillableOutputFallsThroughToTheError(t *testing.T) {
	runner := &alwaysFailRunner{name: "build", err: ErrArtifactTimeout}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, map[Phase]PhaseRunner{PhaseBuild: runner})
	o.retryConfig.PhaseMaxAttempts = 1
	cr := retryParityCycleRun(o, t)
	cr.workflowConfig.BackfillEnabled = true

	_, _, err := cr.retryPhaseRunner(PhaseBuild, PhaseRequest{}, cr.mainDispatchRetryOpts())

	if !errors.Is(err, ErrArtifactTimeout) {
		t.Fatalf("err = %v, want the artifact timeout when nothing can be backfilled", err)
	}
}
