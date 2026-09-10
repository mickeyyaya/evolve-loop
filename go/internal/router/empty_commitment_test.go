package router

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRouteAfterTriageStopsOnEmptyCommittedSet(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
		t.Fatalf("write triage handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[]}`), 0o644); err != nil {
		t.Fatalf("write triage decision: %v", err)
	}

	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.CommittedCount != 0 {
		t.Fatalf("CommittedCount = %d, want 0", sig.Triage.CommittedCount)
	}
	in := base("triage")
	in.Completed = []string{"scout", "triage"}
	in.Signals = sig
	if got := Route(in, nil); got.NextPhase != PhaseEnd {
		t.Fatalf("Route after empty triage = %q (%s), want %q", got.NextPhase, got.Reason, PhaseEnd)
	}
}

func TestRouteAfterTriageAdvancesWithCommittedTask(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "handoff-triage.json"), []byte(`{"cycle_size":"small"}`), 0o644); err != nil {
		t.Fatalf("write triage handoff: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"task-a"}]}`), 0o644); err != nil {
		t.Fatalf("write triage decision: %v", err)
	}

	sig, err := Digest(ws, []string{"scout", "triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if sig.Triage.CommittedCount != 1 {
		t.Fatalf("CommittedCount = %d, want 1", sig.Triage.CommittedCount)
	}
	in := base("triage")
	in.Completed = []string{"scout", "triage"}
	in.Signals = sig
	if got := Route(in, nil); got.NextPhase == PhaseEnd {
		t.Fatalf("Route after committed triage = %q (%s), want the spine to advance", got.NextPhase, got.Reason)
	}
}
