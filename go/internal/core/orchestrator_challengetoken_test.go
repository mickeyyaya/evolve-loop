package core

import (
	"context"
	"encoding/hex"
	"testing"
)

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

// A caller-supplied token (resume / fleet hand-down) must not be overwritten
// by the mint.
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
