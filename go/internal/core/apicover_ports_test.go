package core

// apicover_ports_test.go — public-API coverage (ADR-0050 Phase 5). Names and
// exercises the exported port symbols in ports.go that apicover flags
// UNCOVERED: the Bridge/Guard/Ledger/Storage interfaces, the
// GuardDecision/GuardInput/BatchAccrual/TriageThroughputEntry DTOs, and the
// LedgerEntry.UnmarshalJSON method. Each test reuses the existing in-package
// fakes (fakeStorage, fakeLedger, fakeBridge) and asserts a real contract
// (Rule 9): satisfaction is proven by binding the fake to the port AND driving
// one method through it.

import (
	"context"
	"encoding/json"
	"testing"
)

// apicoverGuard is the one new double this file introduces: there is no Guard
// implementation inside package core (the concrete guards live in
// internal/guards, which imports core, so core cannot import them back). It
// records the GuardInput it received and returns a caller-supplied
// GuardDecision, letting the test prove the Guard port end-to-end without the
// import cycle.
type apicoverGuard struct {
	gotIn    GuardInput
	decision GuardDecision
}

func (g *apicoverGuard) Name() string { return "apicover-fake" }
func (g *apicoverGuard) Decide(_ context.Context, in GuardInput) GuardDecision {
	g.gotIn = in
	return g.decision
}

// TestStoragePort_SatisfiedByFakeAndRoundTrips names the Storage interface in a
// typed declaration (binding the existing fakeStorage to it), then drives one
// method — WriteState/ReadState — to prove the port is exercised, not just
// declared. Contract: a State written through Storage reads back equal.
func TestStoragePort_SatisfiedByFakeAndRoundTrips(t *testing.T) {
	t.Parallel()
	var s Storage = &fakeStorage{} // *fakeStorage must satisfy the Storage port.
	ctx := context.Background()
	want := State{LastCycleNumber: 41, Version: 2}
	if err := s.WriteState(ctx, want); err != nil {
		t.Fatalf("Storage.WriteState: %v", err)
	}
	got, err := s.ReadState(ctx)
	if err != nil {
		t.Fatalf("Storage.ReadState: %v", err)
	}
	if got.LastCycleNumber != want.LastCycleNumber || got.Version != want.Version {
		t.Errorf("ReadState round-trip = %+v, want LastCycleNumber=41 Version=2", got)
	}
}

// TestLedgerPort_SatisfiedByFakeAndAppends names the Ledger interface, binds the
// existing fakeLedger to it, and drives Append to prove the method runs.
// Contract: an appended entry is observable in the fake's record.
func TestLedgerPort_SatisfiedByFakeAndAppends(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	var l Ledger = led // *fakeLedger must satisfy the Ledger port.
	ctx := context.Background()
	if err := l.Verify(ctx); err != nil {
		t.Fatalf("Ledger.Verify: %v", err)
	}
	if err := l.Append(ctx, LedgerEntry{Cycle: 7, Role: "build", EntrySeq: 1}); err != nil {
		t.Fatalf("Ledger.Append: %v", err)
	}
	if len(led.entries) != 1 || led.entries[0].Cycle != 7 || led.entries[0].Role != "build" {
		t.Errorf("Append did not record entry: %+v", led.entries)
	}
}

// TestBridgePort_SatisfiedByFakeAndLaunches names the Bridge interface, binds
// the existing fakeBridge to it, and drives Launch + Probe. Contract: Launch
// returns the bridge's stdout and the BridgeRequest reaches the impl.
func TestBridgePort_SatisfiedByFakeAndLaunches(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: "ok"}
	var b Bridge = fb // *fakeBridge must satisfy the Bridge port.
	ctx := context.Background()
	resp, err := b.Launch(ctx, BridgeRequest{CLI: "claude-tmux", Cycle: 7})
	if err != nil {
		t.Fatalf("Bridge.Launch: %v", err)
	}
	if resp.Stdout != "ok" {
		t.Errorf("Launch stdout = %q, want %q", resp.Stdout, "ok")
	}
	if fb.gotReq.CLI != "claude-tmux" || fb.gotReq.Cycle != 7 {
		t.Errorf("Launch did not forward request: %+v", fb.gotReq)
	}
	if _, err := b.Probe(ctx); err != nil {
		t.Fatalf("Bridge.Probe: %v", err)
	}
}

// TestGuardPort_DecideBindsInputAndDecision covers THREE flagged symbols in one
// pass: the Guard interface (satisfaction + Decide exercised), GuardInput (the
// typed Decide argument, fields read by the impl), and GuardDecision (the typed
// return, fields asserted). Contract: the GuardInput the caller passes reaches
// Decide, and the GuardDecision it returns flows back unchanged.
func TestGuardPort_DecideBindsInputAndDecision(t *testing.T) {
	t.Parallel()
	want := GuardDecision{Allow: false, Reason: "blocked by apicover guard"}
	impl := &apicoverGuard{decision: want}
	var g Guard = impl // Guard port satisfaction.
	in := GuardInput{
		ToolName:       "Bash",
		ToolInput:      map[string]any{"command": "danger"},
		CWD:            "/proj",
		CycleStatePath: "/proj/.evolve/cycle-state.json",
	}
	if g.Name() != "apicover-fake" {
		t.Fatalf("Guard.Name() = %q, want apicover-fake", g.Name())
	}
	got := g.Decide(context.Background(), in)
	if got.Allow != want.Allow || got.Reason != want.Reason {
		t.Errorf("Decide returned %+v, want %+v", got, want)
	}
	if impl.gotIn.ToolName != "Bash" || impl.gotIn.CWD != "/proj" {
		t.Errorf("GuardInput did not reach Decide: %+v", impl.gotIn)
	}
}

// TestBatchAccrual_BoundViaState binds BatchAccrual through a real producer/
// consumer: State.CurrentBatch round-tripped through the Storage port. Contract:
// the BatchAccrual written in State survives the write/read and its
// CycleAccruedCostUSD/GoalHash fields are intact.
func TestBatchAccrual_BoundViaState(t *testing.T) {
	t.Parallel()
	var s Storage = &fakeStorage{}
	ctx := context.Background()
	batch := BatchAccrual{CycleAccruedCostUSD: 1.25, GoalHash: "goal-abc"}
	if err := s.WriteState(ctx, State{CurrentBatch: batch}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	got, err := s.ReadState(ctx)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if got.CurrentBatch.CycleAccruedCostUSD != 1.25 || got.CurrentBatch.GoalHash != "goal-abc" {
		t.Errorf("CurrentBatch round-trip = %+v, want {1.25 goal-abc}", got.CurrentBatch)
	}
}

// TestTriageThroughputEntry_BoundViaJSON binds TriageThroughputEntry through the
// State JSON schema it lives in (State.TriageThroughput). Contract: a
// throughput entry serialized inside State decodes back with its Cycle/Floors
// fields intact and the documented json tags.
func TestTriageThroughputEntry_BoundViaJSON(t *testing.T) {
	t.Parallel()
	in := State{TriageThroughput: []TriageThroughputEntry{{Cycle: 281, Floors: 5}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal State: %v", err)
	}
	var got State
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal State: %v", err)
	}
	if len(got.TriageThroughput) != 1 {
		t.Fatalf("TriageThroughput len = %d, want 1", len(got.TriageThroughput))
	}
	if e := got.TriageThroughput[0]; e.Cycle != 281 || e.Floors != 5 {
		t.Errorf("entry round-trip = %+v, want {Cycle:281 Floors:5}", e)
	}
}

// TestLedgerEntry_UnmarshalJSON_NamedDirectCall invokes the UnmarshalJSON method
// by NAME on a LedgerEntry value (not via json.Unmarshal), so the identifier is
// both named and executed. Contract: it parses a real ledger line, routing the
// numeric cycle to Cycle and populating the scalar fields.
func TestLedgerEntry_UnmarshalJSON_NamedDirectCall(t *testing.T) {
	t.Parallel()
	line := []byte(`{"ts":"2026-06-16T00:00:00Z","cycle":312,"role":"ship","kind":"phase","exit_code":0,"entry_seq":2100,"prev_hash":"feed"}`)
	var e LedgerEntry
	if err := e.UnmarshalJSON(line); err != nil {
		t.Fatalf("LedgerEntry.UnmarshalJSON: %v", err)
	}
	if e.Cycle != 312 {
		t.Errorf("Cycle = %d, want 312", e.Cycle)
	}
	if e.Role != "ship" || e.Kind != "phase" || e.EntrySeq != 2100 || e.PrevHash != "feed" {
		t.Errorf("scalar fields not parsed: %+v", e)
	}

	// String-cycle form routes to CycleLabel via the same named call.
	var legacy LedgerEntry
	if err := legacy.UnmarshalJSON([]byte(`{"cycle":"manual-release-v19.0.0","role":"auditor"}`)); err != nil {
		t.Fatalf("UnmarshalJSON string cycle: %v", err)
	}
	if legacy.Cycle != 0 || legacy.CycleLabel != "manual-release-v19.0.0" {
		t.Errorf("string cycle = {Cycle:%d Label:%q}, want {0 manual-release-v19.0.0}", legacy.Cycle, legacy.CycleLabel)
	}
}
