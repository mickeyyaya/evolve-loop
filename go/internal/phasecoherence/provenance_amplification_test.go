package phasecoherence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLedger(t *testing.T, root, lines string) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "ledger.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hermeticEnv points the ledger lookup at an empty temp root: CheckProvenance resolves the
// ledger from the environment, so the real .evolve/ledger.jsonl would otherwise leak in.
func hermeticEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", tmpDir)
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", "")
	return tmpDir
}

func TestCheckProvenance_BothTreeSHAAndInputsDigestMismatch(t *testing.T) {
	hermeticEnv(t)

	artifact := "<!-- evolve:provenance phase=build cycle=242 tree_sha=wrongsha inputs_digest=wrongdigest -->\n# Report"
	expected := ProvenanceFields{
		Phase: "build", Cycle: 242,
		TreeSHA: "correctsha", InputsDigest: "correctdigest0000",
	}
	violations := CheckProvenance(artifact, expected)

	var hasTree, hasDigest bool
	for _, v := range violations {
		if v.Kind == "provenance-mismatch" && strings.Contains(v.Message, "tree_sha") {
			hasTree = true
		}
		if v.Kind == "provenance-mismatch" && strings.Contains(v.Message, "inputs_digest") {
			hasDigest = true
		}
	}
	if !hasTree {
		t.Errorf("expected tree_sha violation not found in: %+v", violations)
	}
	if !hasDigest {
		t.Errorf("expected inputs_digest violation not found in: %+v", violations)
	}
}

func TestCheckProvenance_TreeSHAMismatchOnly_NoLedger(t *testing.T) {
	hermeticEnv(t)

	artifact := "<!-- evolve:provenance phase=build cycle=242 tree_sha=badsha inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase: "build", Cycle: 242,
		TreeSHA: "goodsha", InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.Severity != "error" {
		t.Errorf("expected Severity error, got %q", v.Severity)
	}
	if v.Kind != "provenance-mismatch" {
		t.Errorf("expected Kind provenance-mismatch, got %q", v.Kind)
	}
	if !strings.Contains(v.Message, "tree_sha") {
		t.Errorf("expected tree_sha in message, got %q", v.Message)
	}
}

func TestCheckProvenance_EmptyExpectedTreeSHA_NoViolation(t *testing.T) {
	hermeticEnv(t)

	artifact := "<!-- evolve:provenance phase=build cycle=242 tree_sha=anysha inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase: "build", Cycle: 242,
		TreeSHA: "", InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 0 {
		t.Errorf("expected 0 violations for empty expected.TreeSHA, got %d: %+v", len(violations), violations)
	}
}

func TestCheckProvenance_ValidHeaderAllFields(t *testing.T) {
	hermeticEnv(t)

	artifact := "<!-- evolve:provenance phase=build cycle=242 tree_sha=abc123 inputs_digest=dig456 -->\n# Report"
	expected := ProvenanceFields{
		Phase: "build", Cycle: 242,
		TreeSHA: "abc123", InputsDigest: "dig456",
	}
	violations := CheckProvenance(artifact, expected)

	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestCheckProvenance_LedgerAndDirectMismatch_SingleTreeSHAViolation(t *testing.T) {
	tmpDir := hermeticEnv(t)

	writeLedger(t, tmpDir, `{"cycle": 242, "role": "build", "tree_state_sha": "goodsha"}`+"\n")

	artifact := "<!-- evolve:provenance phase=build cycle=242 tree_sha=badsha inputs_digest=digest789 -->\n# Report"
	expected := ProvenanceFields{
		Phase: "build", Cycle: 242,
		TreeSHA: "goodsha", InputsDigest: "digest789",
	}
	violations := CheckProvenance(artifact, expected)

	treeCount := 0
	for _, v := range violations {
		if v.Kind == "provenance-mismatch" && strings.Contains(v.Message, "tree_sha") {
			treeCount++
		}
	}
	if treeCount != 1 {
		t.Errorf("expected exactly 1 tree_sha violation (dedup), got %d: %+v", treeCount, violations)
	}
	if len(violations) != 1 {
		t.Errorf("expected 1 total violation, got %d: %+v", len(violations), violations)
	}
}
