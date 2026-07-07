package dossier

// sweep_result_test.go — apicover naming + field-semantics test for the
// SweepResult type (flagged UNCOVERED: sweep_test.go exercises SweepOrphans
// but never names the result type in its AST). One pass pins all three
// fields' meanings against a real repo: a complete orphan pair is
// Recommitted, a lone half-pair is Skipped, and a clean run leaves Failed
// empty but non-nil (safe to range/assign into).
import (
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestSweepResult_FieldsReportOnePassOutcome(t *testing.T) {
	dir := t.TempDir()
	initSweepRepo(t, dir)
	writePairFile(t, dir, "cycle-7.json", `{"cycle":7}`)
	writePairFile(t, dir, "cycle-7.md", "# cycle 7\n")
	writePairFile(t, dir, "cycle-8.json", `{"cycle":8}`) // lone half-pair

	// Explicitly typed (not just `:=`): apicover requires the SweepResult
	// identifier to appear in this package's test AST — collapsing this into
	// the short declaration would re-flag the type as UNCOVERED.
	var got SweepResult
	got, err := SweepOrphans(gitexec.Default(dir), io.Discard)
	if err != nil {
		t.Fatalf("SweepOrphans: %v", err)
	}
	if len(got.Recommitted) != 1 || got.Recommitted[0] != 7 {
		t.Errorf("Recommitted = %v, want [7] (the complete pair)", got.Recommitted)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != 8 {
		t.Errorf("Skipped = %v, want [8] (the lone half-pair)", got.Skipped)
	}
	if got.Failed == nil || len(got.Failed) != 0 {
		t.Errorf("Failed = %v, want empty non-nil map on a clean pass", got.Failed)
	}
}
