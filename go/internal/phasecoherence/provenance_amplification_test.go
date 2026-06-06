package phasecoherence

// Cycle-242 amplification tests — encode the cycle-241 audit HIGH finding:
// CheckProvenance only compared tree_sha against the ledger, never directly
// against expected.TreeSHA, so a bad tree_sha with no ledger produced zero
// violations. These tests pin the direct check AND the dedup contract
// (exactly one tree_sha violation when both the direct check and the ledger
// cross-check would fire — scout B2 risk; TestProvenanceGate_LedgerCrossCheck
// already requires len==1 for that scenario).
//
// TDD contract (cycle 242): Builder makes these pass WITHOUT modifying them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLedger materializes <root>/.evolve/ledger.jsonl with the given lines.
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

// hermeticEnv points the ledger lookup at an empty temp project root so the
// real .evolve/ledger.jsonl can never leak into a test (CheckProvenance
// resolves the ledger via paths.Resolve(os.Getenv, "")).
func hermeticEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("EVOLVE_PROJECT_ROOT", tmpDir)
	t.Setenv("EVOLVE_LEDGER_OVERRIDE", "")
	return tmpDir
}

// AC1 (primary, audit HIGH): both tree_sha and inputs_digest mismatches must
// each fire a violation — with NO ledger present.
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

// tree_sha mismatch ALONE (digest matches, no ledger) must fire exactly one
// error violation — the direct-check gap in its purest form.
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

// Edge: empty expected.TreeSHA means "no expectation" — the direct check must
// NOT fire (mirrors the existing inputs_digest guard semantics).
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

// Negative/anti-over-fix: a fully matching header must stay at zero
// violations after the direct check lands.
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

// Dedup contract (scout B2): when a ledger entry exists AND agrees with
// expected.TreeSHA, a mismatched artifact tree_sha must yield exactly ONE
// tree_sha violation — not one from the direct check plus one from the
// ledger cross-check. This is the same invariant the pre-existing
// TestProvenanceGate_LedgerCrossCheck (len==1) enforces; restated here so
// the dedup requirement is explicit in the cycle-242 contract.
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
