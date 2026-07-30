package dossier

// read_test.go — ReadCommitted's own contract. Its production caller is the loop
// summary's spine fail-open roll-up (cmd/evolve/cmd_loop_outcome.go
// spineFailOpenRollup), which needs exactly three properties from this reader: the
// window is honored (a batch must not fold history), the cycle order is
// deterministic, and one broken file cannot take the reporting surface down.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func writeCorpusDossier(t *testing.T, root string, cycle int, events []cyclestate.SpineFailOpen) {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(&Dossier{
		Cycle:          cycle,
		Goal:           "corpus fixture",
		FinalVerdict:   VerdictPass,
		Phases:         []PhaseRecord{{Name: "cycle-recorded", Verdict: VerdictPass}},
		SpineFailOpens: events,
	})
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, fmt.Sprintf("cycle-%d.json", cycle))
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestReadCommitted_WindowedAscendingAndFaultTolerant covers all three properties
// in one pass over one corpus, because they are one contract: what a batch-scoped
// reporting surface can safely trust.
func TestReadCommitted_WindowedAscendingAndFaultTolerant(t *testing.T) {
	root := t.TempDir()
	writeCorpusDossier(t, root, 9, nil)  // before the window
	writeCorpusDossier(t, root, 11, nil) // in window
	writeCorpusDossier(t, root, 10, []cyclestate.SpineFailOpen{{Phase: "ship", MissingArtifact: "build"}})
	dir := filepath.Join(root, "knowledge-base", "cycles")
	// Junk the reader must skip rather than fail on: unparseable JSON, a non-dossier
	// filename, and the .md half of a pair.
	if err := os.WriteFile(filepath.Join(dir, "cycle-12.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.json"), []byte(`{"cycle":13}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-11.md"), []byte("# md half"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ReadCommitted(root, 10)
	if len(got) != 2 {
		t.Fatalf("ReadCommitted(minCycle=10) returned %d dossiers, want 2 (cycle 9 is outside the "+
			"window; the malformed and non-dossier files are skipped, never fatal)", len(got))
	}
	if got[0].Cycle != 10 || got[1].Cycle != 11 {
		t.Errorf("cycles = [%d %d], want ascending [10 11] — a reporting surface's order must be deterministic", got[0].Cycle, got[1].Cycle)
	}
	if len(got[0].SpineFailOpens) != 1 || got[0].SpineFailOpens[0].MissingArtifact != "build" {
		t.Errorf("cycle 10 spine fail-opens = %+v, want the committed record round-tripped", got[0].SpineFailOpens)
	}

	// An absent corpus is not an error: a fresh project has written no dossiers.
	if got := ReadCommitted(t.TempDir(), 1); got != nil {
		t.Errorf("absent knowledge-base/cycles returned %+v, want nil", got)
	}
	// minCycle <= 0 reads everything — callers that cannot bound their window must
	// decide for themselves whether that is what they want.
	if got := ReadCommitted(root, 0); len(got) != 3 {
		t.Errorf("ReadCommitted(minCycle=0) returned %d, want all 3 parseable dossiers", len(got))
	}
}
