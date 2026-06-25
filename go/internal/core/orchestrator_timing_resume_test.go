//go:build integration

// HIGH-1 regression (integration tier — reuses the git-backed initBindingRepo
// harness, same as the sibling resume tests).
package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The --resume path (RunCycleFromPhase) is a first-class dispatch surface and
// must stamp started_at too — a post-crash resume is exactly when latency
// evidence matters most. Before the fix, cs.PhaseStartedAt was never set on
// resume, so every resumed phase wrote an empty started_at.
func TestPhaseTiming_ResumeStampsStart(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-7")
	st := &fakeStorage{
		state:      State{LastCycleNumber: 7},
		cycleState: CycleState{CycleID: 7, WorkspacePath: ws},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 7}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ws, "phase-timing.json"))
	if err != nil {
		t.Fatalf("phase-timing.json must exist after resume: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("phase-timing.json must be valid JSON: %v\n%s", err, data)
	}
	var audit map[string]any
	for _, e := range entries {
		if e["phase"] == "audit" {
			audit = e
		}
	}
	if audit == nil {
		t.Fatalf("no audit timing entry after resume; got %v", entries)
	}
	if s, _ := audit["started_at"].(string); s == "" {
		t.Errorf("resumed audit phase must carry a non-empty started_at; got %v", audit)
	}
}
