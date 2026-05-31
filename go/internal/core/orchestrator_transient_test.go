package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func wrapTransient(code int) error {
	if code == 81 {
		return errArtifactTimeoutWrapper{}
	}
	if code == 80 || code == 85 || code == 86 {
		return errTransientWrapper{code: code}
	}
	return errGenericExit{code: code}
}

type errTransientWrapper struct {
	code int
}

func (e errTransientWrapper) Error() string {
	return "bridge: launch exit=" + strconv.Itoa(e.code) + ": " + ErrTransientBridgeFailure.Error()
}

func (e errTransientWrapper) Unwrap() error {
	return ErrTransientBridgeFailure
}

type errGenericExit struct {
	code int
}

func (e errGenericExit) Error() string {
	return "bridge: launch exit=" + strconv.Itoa(e.code)
}

func TestIsTransientBridgeError(t *testing.T) {
	if !isTransientBridgeError(wrapTransient(80)) {
		t.Error("exit 80 should be transient")
	}
	if !isTransientBridgeError(wrapTransient(85)) {
		t.Error("exit 85 should be transient")
	}
	if !isTransientBridgeError(wrapTransient(86)) {
		t.Error("exit 86 should be transient")
	}
	if isTransientBridgeError(wrapTransient(81)) {
		t.Error("exit 81 (ArtifactTimeout) should not be classified as transient bridge failure sentinel")
	}
	if isTransientBridgeError(wrapTransient(2)) {
		t.Error("exit 2 should not be transient")
	}
}

func TestOrchestrator_RetryOnTransientExit(t *testing.T) {
	for _, code := range []int{80, 85, 86} {
		t.Run("exit-"+strconv.Itoa(code), func(t *testing.T) {
			st := &fakeStorage{state: State{LastCycleNumber: 0}}
			led := &fakeLedger{}
			runners := buildRunners(nil)
			runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(code), failUntil: 1}
			o := NewOrchestrator(st, led, runners)

			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if got := runners[PhaseScout].(*fakeRunner).calls; got != 2 {
				t.Errorf("calls=%d, want 2", got)
			}
			if res.FinalVerdict != VerdictPASS {
				t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
			}
		})
	}
}

type nonCanonicalRunner struct {
	calls int
}

func (n *nonCanonicalRunner) Name() string { return "scout" }
func (n *nonCanonicalRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	n.calls++
	if n.calls == 1 {
		return PhaseResponse{Phase: "scout", Verdict: "BLAH", ArtifactsDir: req.Workspace}, nil
	}
	return PhaseResponse{Phase: "scout", Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestOrchestrator_NonCanonicalVerdictRetry(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &nonCanonicalRunner{}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}
	if got := runners[PhaseScout].(*nonCanonicalRunner).calls; got != 2 {
		t.Errorf("calls=%d, want 2", got)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
}

func TestOrchestrator_NonTransientError_NoRetry(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(2), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected RunCycle to fail")
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 1 {
		t.Errorf("calls=%d, want 1", got)
	}
}

func TestTransientRetry_Exhausted_WritesFailureDiag(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(80), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})
	if err == nil {
		t.Fatal("expected RunCycle to fail")
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 2 {
		t.Errorf("calls=%d, want 2", got)
	}

	diagPath := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(res.Cycle), "scout-failure-diag.json")
	if _, err := os.Stat(diagPath); err != nil {
		t.Errorf("scout-failure-diag.json not written: %v", err)
	}
}

func TestFAILVerdict_NotRetried(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", verdict: VerdictFAIL}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 1 {
		t.Errorf("calls=%d, want 1", got)
	}
}

func TestTransientRetry_LedgerEntry(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(85), failUntil: 1}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle failed: %v", err)
	}

	var retryEntry *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == "phase_retry" {
			retryEntry = &led.entries[i]
			break
		}
	}
	if retryEntry == nil {
		t.Fatal("expected kind=phase_retry ledger entry")
	}
	if retryEntry.ExitCode != 85 {
		t.Errorf("exit_code=%d, want 85", retryEntry.ExitCode)
	}
}

func TestPhaseMaxAttempts_Default(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(80), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		Env:         map[string]string{},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 2 {
		t.Errorf("calls=%d, want 2", got)
	}
}

func TestPhaseMaxAttempts_EnvOverride(t *testing.T) {
	{
		st := &fakeStorage{state: State{LastCycleNumber: 0}}
		led := &fakeLedger{}
		runners := buildRunners(nil)
		runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(80), failUntil: 99}
		o := NewOrchestrator(st, led, runners)

		_, err := o.RunCycle(context.Background(), CycleRequest{
			ProjectRoot: t.TempDir(),
			Env:         map[string]string{"EVOLVE_PHASE_MAX_ATTEMPTS": "3"},
		})
		if err == nil {
			t.Fatal("expected failure")
		}
		if got := runners[PhaseScout].(*fakeRunner).calls; got != 3 {
			t.Errorf("calls=%d, want 3", got)
		}
	}
	{
		st := &fakeStorage{state: State{LastCycleNumber: 0}}
		led := &fakeLedger{}
		runners := buildRunners(nil)
		runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(80), failUntil: 99}
		o := NewOrchestrator(st, led, runners)

		_, err := o.RunCycle(context.Background(), CycleRequest{
			ProjectRoot: t.TempDir(),
			Env:         map[string]string{"EVOLVE_PHASE_MAX_ATTEMPTS": "5"},
		})
		if err == nil {
			t.Fatal("expected failure")
		}
		if got := runners[PhaseScout].(*fakeRunner).calls; got != 5 {
			t.Errorf("calls=%d, want 5", got)
		}
	}
}

func TestPhaseMaxAttempts_OutOfRange(t *testing.T) {
	cases := []struct {
		val  string
		want int
	}{
		{"0", 2},
		{"6", 5},
		{"", 2},
		{"abc", 2},
	}
	for _, tc := range cases {
		t.Run("val-"+tc.val, func(t *testing.T) {
			st := &fakeStorage{state: State{LastCycleNumber: 0}}
			led := &fakeLedger{}
			runners := buildRunners(nil)
			runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(80), failUntil: 99}
			o := NewOrchestrator(st, led, runners)

			_, err := o.RunCycle(context.Background(), CycleRequest{
				ProjectRoot: t.TempDir(),
				Env:         map[string]string{"EVOLVE_PHASE_MAX_ATTEMPTS": tc.val},
			})
			if err == nil {
				t.Fatal("expected failure")
			}
			if got := runners[PhaseScout].(*fakeRunner).calls; got != tc.want {
				t.Errorf("val=%s: calls=%d, want %d", tc.val, got, tc.want)
			}
		})
	}
}

func TestPhaseMaxAttempts_NonTransient_NoExtraAttempt(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: wrapTransient(2), failUntil: 99}
	o := NewOrchestrator(st, led, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		Env:         map[string]string{"EVOLVE_PHASE_MAX_ATTEMPTS": "5"},
	})
	if err == nil {
		t.Fatal("expected failure")
	}
	if got := runners[PhaseScout].(*fakeRunner).calls; got != 1 {
		t.Errorf("calls=%d, want 1", got)
	}
}
