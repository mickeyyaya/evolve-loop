package core

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestMergeVerdict(t *testing.T) {
	t.Parallel()
	cases := []struct{ acc, v, want string }{
		{VerdictPASS, VerdictPASS, VerdictPASS},
		{VerdictPASS, VerdictWARN, VerdictWARN},
		{VerdictWARN, VerdictPASS, VerdictWARN},
		{VerdictPASS, VerdictFAIL, VerdictFAIL},
		{VerdictWARN, VerdictFAIL, VerdictFAIL},
		{VerdictFAIL, VerdictPASS, VerdictFAIL}, // FAIL is sticky
	}
	for _, c := range cases {
		if got := mergeVerdict(c.acc, c.v); got != c.want {
			t.Errorf("mergeVerdict(%s,%s)=%s, want %s", c.acc, c.v, got, c.want)
		}
	}
}

// concurrentRunner records the peak number of runners executing simultaneously,
// so a batch test can PROVE the phases overlapped rather than ran serially.
type concurrentRunner struct {
	name              string
	active, maxActive *int32
	verdict           string
	err               error
}

func (r *concurrentRunner) Name() string { return r.name }
func (r *concurrentRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	n := atomic.AddInt32(r.active, 1)
	for {
		m := atomic.LoadInt32(r.maxActive)
		if n <= m || atomic.CompareAndSwapInt32(r.maxActive, m, n) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond) // hold the slot so a concurrent peer overlaps
	atomic.AddInt32(r.active, -1)
	if r.err != nil {
		return PhaseResponse{}, r.err
	}
	v := r.verdict
	if v == "" {
		v = VerdictPASS
	}
	return PhaseResponse{Phase: r.name, Verdict: v, ArtifactsDir: req.Workspace}, nil
}

// newBatchCycleRun wires a minimal cycleRun for dispatchEvaluateBatch unit tests.
func newBatchCycleRun(t *testing.T, runners map[Phase]PhaseRunner, conc int) *cycleRun {
	t.Helper()
	root := t.TempDir()
	ws := cycleWorkspaceDir(root, 1)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	o.cfg.ParallelEvaluateConcurrency = conc
	return &cycleRun{
		o:           o,
		ctx:         context.Background(),
		cycle:       1,
		req:         CycleRequest{ProjectRoot: root},
		cs:          CycleState{WorkspacePath: ws, RunID: "r"},
		envSnap:     map[string]string{},
		ctxSnap:     map[string]string{},
		retryConfig: o.retryConfig,
	}
}

func batchHas(cr *cycleRun, phase string) bool {
	for _, p := range cr.result.PhasesRun {
		if string(p) == phase {
			return true
		}
	}
	return false
}

// The two evaluate phases must run CONCURRENTLY (maxActive ≥ 2), both record
// outcomes, and a clean batch merges to PASS + loopNext.
func TestDispatchEvaluateBatch_Concurrent(t *testing.T) {
	t.Parallel()
	var active, maxActive int32
	runners := buildRunners(nil)
	runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &active, maxActive: &maxActive}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &active, maxActive: &maxActive}
	cr := newBatchCycleRun(t, runners, 2)

	act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})
	if err != nil || act != loopNext {
		t.Fatalf("clean batch: act=%v err=%v, want loopNext/nil", act, err)
	}
	if maxActive < 2 {
		t.Errorf("phases did not overlap (maxActive=%d, want ≥2) — they ran serially", maxActive)
	}
	if !batchHas(cr, "tester") || !batchHas(cr, "evaluator") {
		t.Errorf("both phases must be in PhasesRun; got %v", cr.result.PhasesRun)
	}
	if len(cr.phaseTimings) != 2 {
		t.Errorf("want 2 timing entries, got %d", len(cr.phaseTimings))
	}
	if cr.lastVerdict != VerdictPASS {
		t.Errorf("clean batch verdict=%s, want PASS", cr.lastVerdict)
	}
}

// Weakest-link: one WARN in the batch ⇒ the batch verdict is WARN.
func TestDispatchEvaluateBatch_WeakestLink(t *testing.T) {
	t.Parallel()
	var a, m int32
	runners := buildRunners(nil)
	runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &a, maxActive: &m, verdict: VerdictPASS}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &a, maxActive: &m, verdict: VerdictWARN}
	cr := newBatchCycleRun(t, runners, 2)

	act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})
	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v, want loopNext/nil", act, err)
	}
	if cr.lastVerdict != VerdictWARN {
		t.Errorf("weakest-link verdict=%s, want WARN", cr.lastVerdict)
	}
}

// Crash-safety: one runner hard-errors ⇒ all-or-nothing abort, but EVERY phase's
// outcome is still recorded first (C1-complete — no silent loss).
func TestDispatchEvaluateBatch_CrashSafety(t *testing.T) {
	t.Parallel()
	var a, m int32
	runners := buildRunners(nil)
	runners[Phase("tester")] = &concurrentRunner{name: "tester", active: &a, maxActive: &m}
	runners[Phase("evaluator")] = &concurrentRunner{name: "evaluator", active: &a, maxActive: &m, err: errors.New("boom")}
	cr := newBatchCycleRun(t, runners, 2)

	act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})
	if act != loopAbort || err == nil {
		t.Fatalf("a hard error must abort the batch; got act=%v err=%v", act, err)
	}
	// C1-complete: both ran and both were recorded BEFORE the abort.
	if !batchHas(cr, "tester") || !batchHas(cr, "evaluator") {
		t.Errorf("all batch outcomes must be recorded even on abort; PhasesRun=%v", cr.result.PhasesRun)
	}
	// Both batch phases must have a timing entry (a retro entry from
	// failure-learning may also be present — that's the abort path firing).
	n := 0
	for _, e := range cr.phaseTimings {
		if e.Phase == "tester" || e.Phase == "evaluator" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("both batch phases must record a timing entry on abort; got %d", n)
	}
}
