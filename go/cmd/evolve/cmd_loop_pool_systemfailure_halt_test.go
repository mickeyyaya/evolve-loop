package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestDispatchHaltDecision_HaltsOnSystemFailureLane(t *testing.T) {
	results := []fleet.Result{
		{Index: 0, ExitCode: 0},
		{Index: 1, ExitCode: systemFailureHaltExitCode},
		{Index: 2, ExitCode: 2}, // an ordinary FAIL alongside the halt lane
	}
	rc, stopReason, halt := dispatchHaltDecision(results)
	if !halt {
		t.Fatalf("dispatchHaltDecision(halt-code lane present) halt=false, want true — the pool batch must stop on a forged verdict")
	}
	if rc != systemFailureHaltExitCode {
		t.Errorf("rc = %d, want %d (systemFailureHaltExitCode)", rc, systemFailureHaltExitCode)
	}
	if stopReason != "system_failure_halt" {
		t.Errorf("stopReason = %q, want %q", stopReason, "system_failure_halt")
	}
}

func TestDispatchHaltDecision_OrdinaryFailuresContinue(t *testing.T) {
	results := []fleet.Result{
		{Index: 0, ExitCode: 2},
		{Index: 1, ExitCode: 1},
		{Index: 2, ExitCode: -1, Err: errTestLaneFailed},
		{Index: 3, ExitCode: 0},
	}
	rc, stopReason, halt := dispatchHaltDecision(results)
	if halt {
		t.Fatalf("dispatchHaltDecision(ordinary failures only) halt=true, want false — ordinary lane failures must NOT stop the batch")
	}
	if rc != 0 {
		t.Errorf("rc = %d, want 0 (batch continues)", rc)
	}
	if stopReason != "" {
		t.Errorf("stopReason = %q, want empty (batch continues)", stopReason)
	}
}

func TestDispatchHaltDecision_EmptyResultsContinue(t *testing.T) {
	for _, results := range [][]fleet.Result{nil, {}} {
		rc, stopReason, halt := dispatchHaltDecision(results)
		if halt || rc != 0 || stopReason != "" {
			t.Errorf("dispatchHaltDecision(%v) = (rc=%d, stop=%q, halt=%v), want (0, \"\", false)",
				results, rc, stopReason, halt)
		}
	}
}
