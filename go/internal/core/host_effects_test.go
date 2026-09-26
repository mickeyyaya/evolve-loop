package core

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recordingEffects struct {
	log *[]string
	ins []ReviewInput
	err error
}

func (r *recordingEffects) Perform(_ context.Context, in ReviewInput) error {
	*r.log = append(*r.log, "perform:"+in.Phase)
	r.ins = append(r.ins, in)
	return r.err
}

// orderedReviewer rejects each phase in reject that many times.
type orderedReviewer struct {
	log    *[]string
	ins    []ReviewInput
	reject map[string]int
}

func (r *orderedReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	*r.log = append(*r.log, "review:"+in.Phase)
	r.ins = append(r.ins, in)
	if r.reject[in.Phase] > 0 {
		r.reject[in.Phase]--
		return ReviewResult{Reason: "again"}
	}
	return ReviewResult{Approve: true}
}

func assertEffectsPrecedeEveryReview(t *testing.T, log []string, effects *recordingEffects, rev *orderedReviewer) {
	t.Helper()
	for i, entry := range log {
		if phase, ok := strings.CutPrefix(entry, "review:"); ok && (i == 0 || log[i-1] != "perform:"+phase) {
			t.Fatalf("review of %s at %d was not preceded by its host effects: %v", phase, i, log)
		}
	}
	if len(rev.ins) == 0 || !reflect.DeepEqual(effects.ins, rev.ins) {
		t.Fatalf("host effects must see exactly the reviewer's inputs:\n effects=%d reviews=%d", len(effects.ins), len(rev.ins))
	}
}

func TestPerformEffectsAndReview_PerformsHostEffectsBeforeTheReview(t *testing.T) {
	var log []string
	effects := &recordingEffects{log: &log}
	rev := &orderedReviewer{log: &log}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithHostEffects(effects), WithReviewer(rev))
	in := ReviewInput{Cycle: 7, Phase: string(PhaseTriage), Workspace: "/ws", ProjectRoot: "/project"}
	if rr := o.performEffectsAndReview(context.Background(), in); !rr.Approve {
		t.Fatalf("review = %+v", rr)
	}
	if want := []string{"perform:triage", "review:triage"}; !reflect.DeepEqual(log, want) {
		t.Fatalf("log = %v, want %v", log, want)
	}
	assertEffectsPrecedeEveryReview(t, log, effects, rev)
	if !o.HostEffectsWired() {
		t.Fatal("HostEffectsWired must report the bound performer")
	}
}

func TestPerformEffectsAndReview_AFailedHostEffectIsACodedWarningAndStillReviewed(t *testing.T) {
	var log []string
	center := signalcenter.New()
	var got []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	effects := &recordingEffects{log: &log, err: errors.New("inbox unreadable")}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithHostEffects(effects), WithReviewer(&orderedReviewer{log: &log}), WithSignalCenter(center))
	o.performEffectsAndReview(context.Background(), ReviewInput{Cycle: 7, Phase: string(PhaseTriage)})
	if len(log) != 2 || log[1] != "review:triage" {
		t.Fatalf("the review must still run: %v", log)
	}
	if len(got) != 1 || got[0].Code != CodeHostEffectFailed || got[0].Severity != signalcenter.SeverityWarn || got[0].Cycle != 7 || got[0].Phase != "triage" || !strings.Contains(got[0].Reason, "inbox unreadable") {
		t.Fatalf("signals = %+v, want one WARN %s for triage in cycle 7 carrying the cause", got, CodeHostEffectFailed)
	}
}

func TestWithHostEffects_ANilHostEffectsIsUnwired(t *testing.T) {
	var log []string
	var none HostEffects
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithHostEffects(none), WithReviewer(&orderedReviewer{log: &log}))
	o.performEffectsAndReview(context.Background(), ReviewInput{Phase: string(PhaseTriage)})
	if o.HostEffectsWired() || !reflect.DeepEqual(log, []string{"review:triage"}) {
		t.Fatalf("wired=%v log=%v", o.HostEffectsWired(), log)
	}
}

func TestPerformEffectsAndReview_WithoutHostEffectsOnlyReviews(t *testing.T) {
	var log []string
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithReviewer(&orderedReviewer{log: &log}))
	o.performEffectsAndReview(context.Background(), ReviewInput{Phase: string(PhaseTriage)})
	if o.HostEffectsWired() || !reflect.DeepEqual(log, []string{"review:triage"}) {
		t.Fatalf("log = %v", log)
	}
}

func TestRunCycle_EveryReviewPerformsHostEffectsFirst(t *testing.T) {
	t.Parallel()
	var log []string
	effects := &recordingEffects{log: &log}
	rev := &orderedReviewer{log: &log, reject: map[string]int{string(PhaseTriage): 1}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithHostEffects(effects), WithReviewer(rev))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "host-effects"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if n := strings.Count(strings.Join(log, " "), "review:triage"); n != 2 {
		t.Fatalf("triage must be reviewed twice (the rejection, then the correction's re-review): %v", log)
	}
	assertEffectsPrecedeEveryReview(t, log, effects, rev)
}

func TestRunCycleFromPhase_EveryReviewPerformsHostEffectsFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := &fakeWorktree{path: t.TempDir()}
	storage := &fakeStorage{
		state:      State{LastCycleNumber: 7},
		cycleState: CycleState{RunID: "original-run", WorkspacePath: ws, ActiveWorktree: worktree.path, CompletedPhases: []string{"scout"}},
	}
	var log []string
	effects := &recordingEffects{log: &log}
	rev := &orderedReviewer{log: &log, reject: map[string]int{string(PhaseTriage): 1}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(worktree), WithHostEffects(effects), WithReviewer(rev))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "host-effects"}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 7}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if n := strings.Count(strings.Join(log, " "), "review:triage"); n != 2 {
		t.Fatalf("the resumed triage must be reviewed twice: %v", log)
	}
	assertEffectsPrecedeEveryReview(t, log, effects, rev)
	if effects.ins[0].Cycle != 7 {
		t.Fatalf("a checkpoint without a cycle id resumes as the resume point's cycle; host effects saw %d", effects.ins[0].Cycle)
	}
}
