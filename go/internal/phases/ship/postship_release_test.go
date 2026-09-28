package ship

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestInboxPromote_UnfinishedClaimReleasedOnShip: on a successful class=cycle
// ship, a claimed-but-not-in-top_n item ("dropped") is released back to the
// inbox root while the committed item ("kept") is promoted to processed/.
func TestInboxPromote_UnfinishedClaimReleasedOnShip(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolve, "inbox")

	mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{"cycle_id": float64(8)})

	runDir := filepath.Join(evolve, "runs", "cycle-8")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "triage-decision.json"),
		[]byte(`{"cycle":8,"top_n":[{"id":"kept"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	procDir := filepath.Join(inbox, "processing", "cycle-8")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "kept.json"), []byte(`{"id":"kept"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "dropped.json"), []byte(`{"id":"dropped"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	if _, err := os.Stat(filepath.Join(inbox, "dropped.json")); err != nil {
		t.Errorf("residual claim 'dropped' not released to inbox root on ship: %v", err)
	}
	if _, err := os.Stat(filepath.Join(procDir, "dropped.json")); err == nil {
		t.Errorf("residual claim 'dropped' still stranded in processing/cycle-8/")
	}
	if _, err := os.Stat(filepath.Join(procDir, "kept.json")); err == nil {
		t.Errorf("committed item 'kept' still in processing/cycle-8/ — should be promoted")
	}
	if _, err := os.Stat(filepath.Join(inbox, "kept.json")); err == nil {
		t.Errorf("committed item 'kept' wrongly released to inbox root — it must go to processed/")
	}
}

// The residual drain must run regardless of triage-decision.json's presence
// (Step 0a reads only inbox/ root).
func TestInboxPromote_NoTriageDecision_StillDrainsClaims(t *testing.T) {
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolve, "inbox")

	mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{"cycle_id": float64(8)})

	procDir := filepath.Join(inbox, "processing", "cycle-8")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "claimed-defect.json"), []byte(`{"id":"claimed-defect"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := &RunResult{}
	if err := promoteInbox(context.Background(), &Options{ProjectRoot: root}, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	// Must be released back to the inbox ROOT (visible to the next triage scan),
	// not stranded in processing/.
	if _, err := os.Stat(filepath.Join(inbox, "claimed-defect.json")); err != nil {
		t.Errorf("claim not released to inbox root when triage-decision.json absent: %v", err)
	}
	if _, err := os.Stat(filepath.Join(procDir, "claimed-defect.json")); err == nil {
		t.Errorf("claim still stranded in processing/cycle-8/ — the production bug")
	}
}
