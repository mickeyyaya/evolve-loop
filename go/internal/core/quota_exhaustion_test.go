package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAllFamiliesQuotaExhausted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		exits []int
		want  bool
	}{
		{"empty", nil, false},
		{"single 85 proves nothing about the chain", []int{85}, false},
		{"both families drained", []int{85, 85}, true},
		{"three attempts all drained", []int{85, 85, 85}, true},
		{"85 then non-quota transient", []int{85, 80}, false},
		{"artifact timeout then 85", []int{81, 85}, false},
		{"85 then hard failure", []int{85, 1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := allFamiliesQuotaExhausted(tc.exits); got != tc.want {
				t.Errorf("allFamiliesQuotaExhausted(%v)=%v, want %v", tc.exits, got, tc.want)
			}
		})
	}
}

// seqFailRunner returns errs[i] on the i-th call (nil entry → success).
type seqFailRunner struct {
	name  string
	errs  []error
	calls int
}

func (f *seqFailRunner) Name() string { return f.name }
func (f *seqFailRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return PhaseResponse{}, f.errs[i]
	}
	return PhaseResponse{Phase: f.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// Regression for cycle-656 D2: every attempt exits 85 → the dispatch seam must
// write a quota-likely checkpoint, record the C1 abort reason + ledger entry,
// and abort with ErrAllFamiliesExhausted — not fail forward.
// NOT t.Parallel: it swaps the package-level QuotaBoundaryCheckpointer hook.
func TestRunCycle_AllFamilies85_CheckpointsAndDefers(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	defer func() { QuotaBoundaryCheckpointer = prevHook }()
	var hookCalls int
	var hookPhase string
	QuotaBoundaryCheckpointer = func(cs CycleState, projectRoot string, now time.Time) error {
		hookCalls++
		hookPhase = cs.Phase
		return nil
	}

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("RunCycle: want error, got nil")
	}
	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want errors.Is ErrAllFamiliesExhausted", err)
	}
	var clf *ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Errorf("err=%v, want ErrCycleLevelFailure wrapping (loop recoverable branch consumes it)", err)
	}
	if hookCalls != 1 {
		t.Errorf("QuotaBoundaryCheckpointer calls=%d, want 1", hookCalls)
	}
	if hookPhase != string(PhaseScout) {
		t.Errorf("checkpoint cs.Phase=%q, want %q (resumeFromPhase = the exhausted phase)", hookPhase, PhaseScout)
	}
	found := false
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" {
			found = true
			if e.ExitCode != 85 {
				t.Errorf("ledger exit_code=%d, want 85", e.ExitCode)
			}
		}
	}
	if !found {
		t.Errorf("ledger entries %+v: missing kind=all_families_exhausted", led.entries)
	}
	// Attempts stop at the retry cap — no extra spend past one chain pass.
	if calls := runners[PhaseScout].(*fakeRunner).calls; calls > 3 {
		t.Errorf("scout calls=%d, want <= retry cap (no burn past exhaustion)", calls)
	}
}

// Mixed exit codes (85 then 80) prove NOT all families are quota-drained: the
// existing loud-abort path must run unchanged — no checkpoint, no defer.
// (Single-family 85 followed by a healthy sibling success is already pinned by
// TestOrchestrator_RetryOnTransientExit.)
func TestRunCycle_MixedExitCodes_NoDefer(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	defer func() { QuotaBoundaryCheckpointer = prevHook }()
	var hookCalls int
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error {
		hookCalls++
		return nil
	}

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &seqFailRunner{name: "scout", errs: []error{wrapTransient(85), wrapTransient(80), wrapTransient(80)}}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("RunCycle: want error, got nil")
	}
	if errors.Is(err, ErrAllFamiliesExhausted) {
		t.Errorf("err=%v: mixed exit codes must NOT classify as all-families-exhausted", err)
	}
	if hookCalls != 0 {
		t.Errorf("QuotaBoundaryCheckpointer calls=%d, want 0 (no quota checkpoint on mixed failure)", hookCalls)
	}
	for _, e := range led.entries {
		if e.Kind == "all_families_exhausted" {
			t.Errorf("unexpected all_families_exhausted ledger entry: %+v", e)
		}
	}
	if !strings.Contains(err.Error(), "scout") {
		t.Errorf("err=%v, want the failing phase named", err)
	}
}
