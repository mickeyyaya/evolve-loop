package core

import (
	"context"
	"encoding/json"
	"testing"
)

type apicoverGuard struct {
	gotIn GuardInput
}

func (g *apicoverGuard) Name() string { return "apicover-fake" }
func (g *apicoverGuard) Decide(_ context.Context, in GuardInput) GuardDecision {
	g.gotIn = in
	return GuardDecision{Allow: in.ToolName != "Bash", Reason: "apicover guard saw " + in.ToolName + " in " + in.CWD}
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
	rb := &recoverableBridge{fakeBridge: fakeBridge{stdout: "ok"}}
	var b Bridge = rb
	ctx := context.Background()
	resp, err := LaunchWithFreshSession(ctx, b, BridgeRequest{CLI: "claude-tmux", Cycle: 7})
	if err != nil {
		t.Fatalf("LaunchWithFreshSession: %v", err)
	}
	if resp.Stdout != "ok" || resp.SessionRecoverable {
		t.Errorf("response = stdout %q recoverable %v, want ok and a cleared recoverable flag", resp.Stdout, resp.SessionRecoverable)
	}
	reqs := rb.reqs
	if len(reqs) != 2 {
		t.Fatalf("a recoverable first launch must be retried once, got %d launches", len(reqs))
	}
	if reqs[0].CLI != "claude-tmux" || reqs[0].Cycle != 7 || reqs[0].FreshSession {
		t.Errorf("first launch lost the request: %+v", reqs[0])
	}
	if reqs[1].CLI != "claude-tmux" || reqs[1].Cycle != 7 || !reqs[1].FreshSession {
		t.Errorf("retry must repeat the request on a fresh session: %+v", reqs[1])
	}
	if _, err := b.Probe(ctx); err != nil {
		t.Fatalf("Bridge.Probe: %v", err)
	}
}

type recoverableBridge struct {
	fakeBridge
	reqs []BridgeRequest
}

func (r *recoverableBridge) Launch(ctx context.Context, req BridgeRequest) (BridgeResponse, error) {
	r.reqs = append(r.reqs, req)
	resp, err := r.fakeBridge.Launch(ctx, req)
	resp.SessionRecoverable = len(r.reqs) == 1
	return resp, err
}

func TestGuardPort_DecideBindsInputAndDecision(t *testing.T) {
	t.Parallel()
	impl := &apicoverGuard{}
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
	if got.Allow || got.Reason != "apicover guard saw Bash in /proj" {
		t.Errorf("Decide returned %+v, want a denial that names the tool and directory", got)
	}
	seen := impl.gotIn
	if seen.ToolName != "Bash" || seen.CWD != "/proj" || seen.CycleStatePath != "/proj/.evolve/cycle-state.json" {
		t.Errorf("GuardInput did not reach Decide: %+v", seen)
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
