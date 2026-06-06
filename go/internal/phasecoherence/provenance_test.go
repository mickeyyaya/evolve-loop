package phasecoherence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProvenanceGate_MissingHeader_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "# Some Report\nNo provenance here."
	expected := ProvenanceFields{Phase: "build", Cycle: 241}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != "WARN" {
		t.Errorf("expected Severity WARN, got %q", violations[0].Severity)
	}
	if violations[0].Kind != "missing-provenance" {
		t.Errorf("expected Kind missing-provenance, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_ValidHeader_NoViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=build cycle=241 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestProvenanceGate_TamperedPhase_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=scout cycle=241 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected Severity error, got %q", violations[0].Severity)
	}
	if violations[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_WrongCycle_ReturnsViolation(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	artifact := "<!-- evolve:provenance phase=build cycle=240 tree_sha=abcdef123456 inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "abcdef123456",
		InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Severity != "error" {
		t.Errorf("expected Severity error, got %q", violations[0].Severity)
	}
	if violations[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations[0].Kind)
	}
}

func TestProvenanceGate_LedgerCrossCheck(t *testing.T) {
	tmpDir := t.TempDir()
	evolveDir := filepath.Join(tmpDir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EVOLVE_PROJECT_ROOT", tmpDir)

	ledgerPath := filepath.Join(evolveDir, "ledger.jsonl")
	// write a dummy entry
	ledgerLine := `{"cycle": 241, "role": "build", "tree_state_sha": "goodsha"}` + "\n"
	if err := os.WriteFile(ledgerPath, []byte(ledgerLine), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. matching tree_sha -> passes
	artifact1 := "<!-- evolve:provenance phase=build cycle=241 tree_sha=goodsha inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase:        "build",
		Cycle:        241,
		TreeSHA:      "goodsha",
		InputsDigest: "digest789",
	}
	violations1 := CheckProvenance(artifact1, expected)
	if len(violations1) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations1), violations1)
	}

	// 2. mismatching tree_sha -> error
	artifact2 := "<!-- evolve:provenance phase=build cycle=241 tree_sha=badsha inputs_digest=digest789 -->\n# Report"
	violations2 := CheckProvenance(artifact2, expected)
	if len(violations2) != 1 {
		t.Fatalf("expected 1 violation for bad tree_sha, got %d: %+v", len(violations2), violations2)
	}
	if violations2[0].Severity != "error" {
		t.Errorf("expected Severity error, got %q", violations2[0].Severity)
	}
	if violations2[0].Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", violations2[0].Kind)
	}
}
