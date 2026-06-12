package ship

// postship_release_test.go — RED test for cycle-308 task
// `inbox-promote-on-ship-missing` (ship-success residual drain).
//
// promoteInbox currently promotes only triage-decision.json's top_n[] +
// skip_shipped[] to processed/. Items that were CLAIMED into
// processing/cycle-<N>/ but then dropped from top_n are left stranded in
// processing/ forever. The fix: after promoting top_n, drain the residual
// claims back to the inbox root (via inboxmover.ReleaseCycleProcessing) so the
// next cycle re-triages them. Helpers mustWriteState/writeStateMap/anyContains
// live in postship_unit_test.go (same package).

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

	// Active cycle 8.
	mustWriteState(t, filepath.Join(evolve, "cycle-state.json"), map[string]any{"cycle_id": float64(8)})

	// triage-decision.json commits ONLY "kept" in top_n.
	runDir := filepath.Join(evolve, "runs", "cycle-8")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "triage-decision.json"),
		[]byte(`{"cycle":8,"top_n":[{"id":"kept"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Both items were claimed into processing/cycle-8/.
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

	// The residual (dropped) claim must be released back to the inbox ROOT.
	if _, err := os.Stat(filepath.Join(inbox, "dropped.json")); err != nil {
		t.Errorf("residual claim 'dropped' not released to inbox root on ship: %v", err)
	}
	// And it must no longer linger in processing/cycle-8/.
	if _, err := os.Stat(filepath.Join(procDir, "dropped.json")); err == nil {
		t.Errorf("residual claim 'dropped' still stranded in processing/cycle-8/")
	}
	// The committed item was promoted out of processing/ (to processed/), not
	// released back to the inbox root.
	if _, err := os.Stat(filepath.Join(procDir, "kept.json")); err == nil {
		t.Errorf("committed item 'kept' still in processing/cycle-8/ — should be promoted")
	}
	if _, err := os.Stat(filepath.Join(inbox, "kept.json")); err == nil {
		t.Errorf("committed item 'kept' wrongly released to inbox root — it must go to processed/")
	}
}
