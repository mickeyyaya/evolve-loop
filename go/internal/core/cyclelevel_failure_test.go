package core_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestErrCycleLevelFailure_WrapsCauseForErrorsIs(t *testing.T) {
	t.Parallel()
	cause := errors.New("tmux pane died")
	var err error = &core.ErrCycleLevelFailure{Phase: "build", Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is(wrapped, cause) = false — Unwrap() must expose the original cause")
	}
	var clf *core.ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Fatal("errors.As must recover *core.ErrCycleLevelFailure")
	}
	if clf.Phase != "build" {
		t.Errorf("Phase = %q, want \"build\"", clf.Phase)
	}
	msg := err.Error()
	if !strings.Contains(msg, "build") || !strings.Contains(msg, "tmux pane died") {
		t.Errorf("Error() = %q, want it to mention the phase and the cause", msg)
	}
}

// bridgeDeadRunner models a persistently-dead bridge: every launch fails
// with a transient bridge error, so the orchestrator's self-heal retry
// ladder runs to exhaustion before aborting.
type bridgeDeadRunner struct {
	name  string
	calls int
}

func (r *bridgeDeadRunner) Name() string { return r.name }
func (r *bridgeDeadRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	return core.PhaseResponse{}, fmt.Errorf("bridge launch failed (pane died): %w", core.ErrTransientBridgeFailure)
}

func TestOrchestrator_BridgeExhaustion_CycleLevelFailure(t *testing.T) {
	t.Parallel()
	scout := &bridgeDeadRunner{name: "scout"}
	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseScout: scout,
	}))
	_, err := runCycleT(t, orch)
	if err == nil {
		t.Fatal("a permanently-dead bridge must fail the cycle")
	}
	var clf *core.ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Fatalf("bridge exhaustion must classify CYCLE-level (ErrCycleLevelFailure), got: %v", err)
	}
	if clf.Phase != "scout" {
		t.Errorf("ErrCycleLevelFailure.Phase = %q, want \"scout\"", clf.Phase)
	}
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("the original bridge cause must survive the wrap (errors.Is roundtrip); got: %v", err)
	}
	if scout.calls < 2 {
		t.Errorf("scout launch attempts = %d, want >= 2 (retry ladder must run before cycle-level abort)", scout.calls)
	}
}

// integrityErrRunner fails with the given kernel-integrity sentinel.
type integrityErrRunner struct {
	name string
	err  error
}

func (r *integrityErrRunner) Name() string { return r.name }
func (r *integrityErrRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, fmt.Errorf("kernel says no: %w", r.err)
}

func TestOrchestrator_IntegrityBreach_StillBatchFatal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		sentinel error
	}{
		{"phase_gate_denied", core.ErrPhaseGateFailed},
		{"ledger_chain_broken", core.ErrLedgerChainBroken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
				core.PhaseScout: &integrityErrRunner{name: "scout", err: tc.sentinel},
			}))
			_, err := runCycleT(t, orch)
			if err == nil {
				t.Fatal("integrity breach must fail the cycle")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("sentinel must propagate, got: %v", err)
			}
			var clf *core.ErrCycleLevelFailure
			if errors.As(err, &clf) {
				t.Errorf("integrity breach %v was wrapped in ErrCycleLevelFailure — it must stay BATCH-fatal", tc.sentinel)
			}
		})
	}

	// Lock contention short-circuits before any phase runs; it must also
	// stay batch-fatal (another runner would just hit the same lock).
	t.Run("lock_held", func(t *testing.T) {
		lockedSt := &lockHeldStorage{recStorage: &recStorage{}}
		orch := core.NewOrchestrator(lockedSt, &fakeLedger{}, newRunners(nil))
		_, err := runCycleT(t, orch)
		if err == nil {
			t.Fatal("held lock must fail the run")
		}
		if !errors.Is(err, core.ErrLockHeld) {
			t.Fatalf("want ErrLockHeld, got: %v", err)
		}
		var clf *core.ErrCycleLevelFailure
		if errors.As(err, &clf) {
			t.Error("ErrLockHeld wrapped in ErrCycleLevelFailure — lock contention must stay batch-fatal")
		}
	})
}

// lockHeldStorage decorates recStorage so AcquireLock always refuses.
type lockHeldStorage struct {
	*recStorage
}

func (s *lockHeldStorage) AcquireLock(context.Context) (func() error, error) {
	return nil, core.ErrLockHeld
}

// countingPassRunner is passRunner plus an invocation counter, to bound the
// audit↔ship recovery traversal.
type countingPassRunner struct {
	name  string
	calls int
}

func (r *countingPassRunner) Name() string { return r.name }
func (r *countingPassRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestOrchestrator_RecoveryDepthBudget(t *testing.T) {
	t.Parallel()
	se := core.NewShipError(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, core.StageVerifyClass, "always stale")
	ship := &shipErrorStub{name: "ship", failFirst: 99, errOnFail: se}
	audit := &countingPassRunner{name: "audit"}
	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:  ship,
		core.PhaseAudit: audit,
	}))
	_, err := runCycleT(t, orch)
	if err == nil {
		t.Fatal("a never-resolving ship precondition must fail the cycle once the recovery budget is exhausted")
	}
	// Budget: initial + maxRecoveryDepth(2) recoveries — never more.
	if ship.calls != 3 {
		t.Errorf("ship attempts = %d, want 3 (initial + maxRecoveryDepth=2)", ship.calls)
	}
	if audit.calls > 3 {
		t.Errorf("audit ran %d times — the audit↔ship loop must be capped by the same budget (want <= 3)", audit.calls)
	}
	// Exhausted precondition recovery = cycle-level, not batch-fatal.
	var clf *core.ErrCycleLevelFailure
	if !errors.As(err, &clf) {
		t.Errorf("recovery-budget exhaustion (precondition class) must classify cycle-level (ErrCycleLevelFailure), got: %v", err)
	}
	if !errors.Is(err, se) {
		t.Errorf("the exhausted ShipError must remain recoverable from the returned error; got: %v", err)
	}
}
