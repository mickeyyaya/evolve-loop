package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestSealCycle_ManualSeal_RoleIsOperator(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	sealFixture(t, ev, 201)

	led := &recordingLedger{}
	if _, err := SealCycle(context.Background(), led, sealOpts(ev)); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got != "operator" {
		t.Errorf("manual seal Role = %q, want \"operator\"", got)
	}
}

func TestSealCycle_AutomatedRecovery_RoleIsNotOperator(t *testing.T) {
	t.Parallel()
	ev := t.TempDir()
	sealFixture(t, ev, 202)

	opts := sealOpts(ev)
	opts.AutomatedRecovery = true
	led := &recordingLedger{}
	if _, err := SealCycle(context.Background(), led, opts); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got == "operator" {
		t.Errorf("automated-recovery seal must not carry Role=%q — that would silently gain operator trust for chain verification with no human involved", got)
	}
}

func TestAutosealStaleMarker_SealsWithAutomatedRecoveryRole(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	workspace := sealFixture(t, evolveDir, 203)
	frozen := sealOpts(evolveDir).Now()
	if err := runlease.Write(workspace, runlease.Lease{RunID: "run-203", OwnerPID: 999999}, frozen); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	dead := func(int) bool { return false }

	led := &recordingLedger{}
	_, sealed, err := AutosealStaleMarker(context.Background(), led, sealOpts(evolveDir), dead)
	if err != nil {
		t.Fatalf("AutosealStaleMarker: %v", err)
	}
	if !sealed {
		t.Fatal("dead-owner marker must be auto-sealed")
	}
	if len(led.entries) != 1 {
		t.Fatalf("want exactly one ledger append, got %d", len(led.entries))
	}
	if got := led.entries[0].Role; got == "operator" {
		t.Errorf("AutosealStaleMarker must never write Role=%q — it runs unattended at boot with no human sign-off", got)
	}
}
