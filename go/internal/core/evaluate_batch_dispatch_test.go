package core

import (
	"context"
	"errors"
	"fmt"
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
		{VerdictFAIL, VerdictPASS, VerdictFAIL},
	}
	for _, c := range cases {
		if got := mergeVerdict(c.acc, c.v); got != c.want {
			t.Errorf("mergeVerdict(%s,%s)=%s, want %s", c.acc, c.v, got, c.want)
		}
	}
}

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
	time.Sleep(20 * time.Millisecond)
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

const rendezvousPatience = 30 * time.Second

type rendezvous struct {
	want, arrived int32
	missed        atomic.Bool
	all           chan struct{}
}

func newRendezvous(want int32) *rendezvous {
	return &rendezvous{want: want, all: make(chan struct{})}
}

type rendezvousRunner struct {
	name string
	meet *rendezvous
}

func (r *rendezvousRunner) Name() string { return r.name }
func (r *rendezvousRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if atomic.AddInt32(&r.meet.arrived, 1) == r.meet.want {
		close(r.meet.all)
	}
	select {
	case <-r.meet.all:
		return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
	case <-time.After(rendezvousPatience):
		r.meet.missed.Store(true)
		return PhaseResponse{}, fmt.Errorf("%s waited %s and its batch peers never arrived: the batch ran serially", r.name, rendezvousPatience)
	}
}

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

func TestDispatchEvaluateBatch_Concurrent(t *testing.T) {
	t.Parallel()
	meet := newRendezvous(2)
	runners := buildRunners(nil)
	runners[Phase("tester")] = &rendezvousRunner{name: "tester", meet: meet}
	runners[Phase("evaluator")] = &rendezvousRunner{name: "evaluator", meet: meet}
	cr := newBatchCycleRun(t, runners, 2)

	act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"})
	if err != nil || act != loopNext {
		t.Fatalf("both members must be in flight together: act=%v err=%v, want loopNext/nil", act, err)
	}
	if meet.missed.Load() {
		t.Fatal("a member gave up waiting for its batch peer: the batch ran serially")
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
	if !batchHas(cr, "tester") || !batchHas(cr, "evaluator") {
		t.Errorf("all batch outcomes must be recorded even on abort; PhasesRun=%v", cr.result.PhasesRun)
	}
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
