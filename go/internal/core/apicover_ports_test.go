package core

import (
	"context"
	"encoding/json"
	"testing"
)

// apicoverGuard is a Guard double, because core cannot import internal/guards, which imports core.
type apicoverGuard struct {
	gotIn    GuardInput
	decision GuardDecision
}

func (g *apicoverGuard) Name() string { return "apicover-fake" }
func (g *apicoverGuard) Decide(_ context.Context, in GuardInput) GuardDecision {
	g.gotIn = in
	return g.decision
}

func TestStoragePort_SatisfiedByFakeAndRoundTrips(t *testing.T) {
	t.Parallel()
	var s Storage = &fakeStorage{}
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

func TestLedgerPort_SatisfiedByFakeAndAppends(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	var l Ledger = led
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

func TestBridgePort_SatisfiedByFakeAndLaunches(t *testing.T) {
	t.Parallel()
	fb := &fakeBridge{stdout: "ok"}
	var b Bridge = fb
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

func TestGuardPort_DecideBindsInputAndDecision(t *testing.T) {
	t.Parallel()
	want := GuardDecision{Allow: false, Reason: "blocked by apicover guard"}
	impl := &apicoverGuard{decision: want}
	var g Guard = impl
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

	var legacy LedgerEntry
	if err := legacy.UnmarshalJSON([]byte(`{"cycle":"manual-release-v19.0.0","role":"auditor"}`)); err != nil {
		t.Fatalf("UnmarshalJSON string cycle: %v", err)
	}
	if legacy.Cycle != 0 || legacy.CycleLabel != "manual-release-v19.0.0" {
		t.Errorf("string cycle = {Cycle:%d Label:%q}, want {0 manual-release-v19.0.0}", legacy.Cycle, legacy.CycleLabel)
	}
}
