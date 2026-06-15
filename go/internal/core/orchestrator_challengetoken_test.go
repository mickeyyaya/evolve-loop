package core

import (
	"context"
	"encoding/hex"
	"testing"
)

// orchestrator_challengetoken_test.go — characterization net for the challenge-token
// mint (RunCycle init, orchestrator.go ~line 581). Every cycle mints an 8-byte hex
// token and threads it to every phase via Context["challengeToken"] (scout binds its
// eval to it; PR-5 anti-replay fallback source). This behavior was EXERCISED by every
// cycle test but never ASSERTED — a refactor (the planCycle extraction) could silently
// drop the mint and all other tests would still pass. These tests pin it first.

// TestRunCycle_MintsChallengeToken: a cycle with no caller-supplied token mints a
// fresh 8-byte hex one and passes it to the first phase.
func TestRunCycle_MintsChallengeToken(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	scout := runners[PhaseScout].(*fakeRunner)
	o := NewOrchestrator(st, led, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if len(scout.requests) == 0 {
		t.Fatal("scout was never dispatched")
	}
	tok := scout.requests[0].Context["challengeToken"]
	if tok == "" {
		t.Fatal("Context[challengeToken] not set — mint dropped")
	}
	if raw, err := hex.DecodeString(tok); err != nil || len(raw) != 8 {
		t.Errorf("challengeToken=%q, want 8-byte hex (16 chars); decodeErr=%v len=%d", tok, err, len(raw))
	}
}

// TestRunCycle_PreservesSuppliedChallengeToken: when the caller pre-supplies a token
// (resume / fleet hand-down), the mint must NOT overwrite it.
func TestRunCycle_PreservesSuppliedChallengeToken(t *testing.T) {
	const supplied = "deadbeefdeadbeef"
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	scout := runners[PhaseScout].(*fakeRunner)
	o := NewOrchestrator(st, led, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
		Context: map[string]string{"challengeToken": supplied},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := scout.requests[0].Context["challengeToken"]; got != supplied {
		t.Errorf("challengeToken=%q, want supplied %q (mint must not overwrite)", got, supplied)
	}
}
