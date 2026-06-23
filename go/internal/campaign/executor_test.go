package campaign

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/fleet"
)

func mkOptionalWave(ids ...string) []fleet.CycleSpec {
	specs := mkwave(ids...)
	for i := range specs {
		specs[i].Optional = true
	}
	return specs
}

// recordingRunner is a fake WaveRunner. It records the scopes it was asked to run
// and fails a cycle id either a fixed number of times before succeeding
// (failTimes) or forever (failForever) — enough to exercise retry, retry-then-
// succeed, and retry-exhausted-abort without coupling to call order.
type recordingRunner struct {
	mu          sync.Mutex
	ranScopes   [][]string
	calls       int
	seen        map[string]int
	failTimes   map[string]int
	failForever map[string]bool
}

func (r *recordingRunner) run(_ context.Context, w []fleet.CycleSpec) []fleet.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.seen == nil {
		r.seen = map[string]int{}
	}
	var scopes []string
	res := make([]fleet.Result, len(w))
	for i, s := range w {
		scopes = append(scopes, s.Scope...)
		id := ""
		if len(s.Scope) > 0 {
			id = s.Scope[0]
		}
		r.seen[id]++
		if r.failForever[id] || r.seen[id] <= r.failTimes[id] {
			res[i] = fleet.Result{Index: i, ExitCode: 1}
		} else {
			res[i] = fleet.Result{Index: i, ExitCode: 0}
		}
	}
	r.ranScopes = append(r.ranScopes, scopes)
	return res
}

func mkwave(ids ...string) []fleet.CycleSpec {
	specs := make([]fleet.CycleSpec, len(ids))
	for i, id := range ids {
		specs[i] = fleet.CycleSpec{Scope: []string{id}}
	}
	return specs
}

func TestWaveRunner_MethodValueSatisfiesType(t *testing.T) {
	r := &recordingRunner{}
	var _ WaveRunner = r.run // compile-time: the injected runner type is real
}

func TestRunWaves_ResumeSkipsCompletedWaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	if err := (&CampaignProgress{PlanSHA: "P", CompletedWaves: []int{0, 1}, CompletedCycleIDs: []string{"a", "b"}}).Save(path); err != nil {
		t.Fatal(err)
	}
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b"), mkwave("c")}
	r := &recordingRunner{}
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", Resume: true}); err != nil {
		t.Fatalf("RunWaves: %v", err)
	}
	if r.calls != 1 {
		t.Fatalf("ran %d waves, want 1 (only the uncompleted wave 2)", r.calls)
	}
	if len(r.ranScopes) != 1 || len(r.ranScopes[0]) != 1 || r.ranScopes[0][0] != "c" {
		t.Errorf("ran scopes %v, want only [c]", r.ranScopes)
	}
}

func TestRunWaves_RecordsProgressAfterEachWave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b", "c")}
	r := &recordingRunner{}
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P"}); err != nil {
		t.Fatalf("RunWaves: %v", err)
	}
	got, err := LoadProgress(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsWaveComplete(0) || !got.IsWaveComplete(1) {
		t.Errorf("progress missing completed waves: %+v", got)
	}
	if got.PlanSHA != "P" {
		t.Errorf("PlanSHA = %q, want P", got.PlanSHA)
	}
	if len(got.CompletedCycleIDs) != 3 {
		t.Errorf("CompletedCycleIDs = %v, want a,b,c", got.CompletedCycleIDs)
	}
}

func TestRunWaves_StaleProgressIgnoredOnPlanShaMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	if err := (&CampaignProgress{PlanSHA: "OLD", CompletedWaves: []int{0, 1}}).Save(path); err != nil {
		t.Fatal(err)
	}
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b")}
	r := &recordingRunner{}
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "NEW", Resume: true}); err != nil {
		t.Fatalf("RunWaves: %v", err)
	}
	if r.calls != 2 {
		t.Errorf("ran %d waves, want 2 (stale progress for a different plan ignored)", r.calls)
	}
}

func TestRunWaves_NoRetryStopsAtFirstFailedWave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b"), mkwave("c")}
	r := &recordingRunner{failForever: map[string]bool{"b": true}}
	err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 0})
	if err == nil {
		t.Fatal("RunWaves: want error on failed wave, got nil")
	}
	got, _ := LoadProgress(path)
	if !got.IsWaveComplete(0) {
		t.Error("wave 0 should be marked complete")
	}
	if got.IsWaveComplete(1) {
		t.Error("failed wave 1 must NOT be marked complete")
	}
	if got.IsWaveComplete(2) {
		t.Error("wave 2 must not run after wave 1 aborts")
	}
}

func TestRunWaves_RetriesFailedSpecThenSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a", "b")}
	// "b" fails its first attempt, succeeds on retry; "a" succeeds first try.
	r := &recordingRunner{failTimes: map[string]int{"b": 1}}
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 1}); err != nil {
		t.Fatalf("RunWaves: want success after retry, got %v", err)
	}
	// Retry must be BATCHED to only the failed spec: 1st call runs [a b], retry runs [b].
	if r.calls != 2 {
		t.Fatalf("calls = %d, want 2 (wave + one batched retry of the failed spec)", r.calls)
	}
	if len(r.ranScopes) != 2 || len(r.ranScopes[1]) != 1 || r.ranScopes[1][0] != "b" {
		t.Errorf("retry scopes = %v, want second run to be only [b]", r.ranScopes)
	}
	got, _ := LoadProgress(path)
	if !got.IsWaveComplete(0) {
		t.Error("wave 0 should be complete after a successful retry")
	}
}

func TestRunWaves_StopsPromptlyOnCanceledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // operator interrupt before the wave even settles
	waves := [][]fleet.CycleSpec{mkwave("a")}
	r := &recordingRunner{failForever: map[string]bool{"a": true}}
	err := RunWaves(ctx, waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 3})
	if err == nil {
		t.Fatal("RunWaves: want error on canceled context, got nil")
	}
	if r.calls != 1 {
		t.Errorf("calls = %d, want 1 (a canceled run must not retry)", r.calls)
	}
}

func TestRunWaves_AbortsAfterRetriesExhausted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a")}
	r := &recordingRunner{failForever: map[string]bool{"a": true}}
	err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 1})
	if err == nil {
		t.Fatal("RunWaves: want error after retries exhausted, got nil")
	}
	if r.calls != 2 {
		t.Errorf("calls = %d, want 2 (initial + one retry before abort)", r.calls)
	}
}

func TestRunWaves_OptionalPoisonSkippedAndContinues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkOptionalWave("opt"), mkwave("b")}
	r := &recordingRunner{failForever: map[string]bool{"opt": true}}
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 1}); err != nil {
		t.Fatalf("RunWaves: an optional poison cycle must not abort the campaign, got %v", err)
	}
	prog, _ := LoadProgress(path)
	if !containsStr(prog.FailedCycleIDs, "opt") {
		t.Errorf("optional poison 'opt' not quarantined: FailedCycleIDs=%v", prog.FailedCycleIDs)
	}
	if !prog.IsWaveComplete(1) {
		t.Error("wave 1 (b) must still run after an optional poison in wave 0")
	}
	if containsStr(prog.CompletedCycleIDs, "opt") {
		t.Error("quarantined 'opt' must not be recorded as completed")
	}
}

func TestRunWaves_RequiredPoisonAbortsEvenWithOptionalPeer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	w := append(mkwave("req"), mkOptionalWave("opt")...) // mixed wave, both fail
	r := &recordingRunner{failForever: map[string]bool{"req": true, "opt": true}}
	if err := RunWaves(context.Background(), [][]fleet.CycleSpec{w}, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", MaxRetries: 0}); err == nil {
		t.Fatal("RunWaves: a required poison must abort even alongside an optional failure")
	}
}

func TestRunWaves_CooldownWaitedBeforeRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a")}
	r := &recordingRunner{failTimes: map[string]int{"a": 1}} // fail once, succeed on retry
	var slept []time.Duration
	cool := 50 * time.Millisecond
	err := RunWaves(context.Background(), waves, r.run, RunOptions{
		ProgressPath: path, PlanSHA: "P", MaxRetries: 1,
		Cooldown: func() time.Duration { return cool },
		Sleep:    func(d time.Duration) { slept = append(slept, d) },
	})
	if err != nil {
		t.Fatalf("RunWaves: %v", err)
	}
	if len(slept) != 1 || slept[0] != cool {
		t.Errorf("cooldown sleeps = %v, want exactly one sleep of %v before the retry", slept, cool)
	}
}

func TestRunWaves_BeforeWaveCalledPerRunWave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b")}
	r := &recordingRunner{}
	calls := 0
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{
		ProgressPath: path, PlanSHA: "P",
		BeforeWave: func() { calls++ },
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("BeforeWave called %d times, want 2 (once per run wave)", calls)
	}
}
