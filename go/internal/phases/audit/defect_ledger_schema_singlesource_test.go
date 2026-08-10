package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// defect_ledger_schema_singlesource_test.go — the third leg of the doc-sync
// contract (cycle-1403). AC9 already holds agents/evolve-auditor.md and
// docs/architecture/continuation-defect-ledger.md to each other; this holds the
// GO constant echoed inline on rejection (dispositionSchemaExample) to the same
// document. Without it the two docs could stay in lockstep while the message an
// agent actually reads at the moment of failure drifted away from both — which
// is the failure mode this cycle exists to close, one level down.
func TestDispositionSchemaExampleMatchesDocumentedExample(t *testing.T) {
	root := docExampleRepoRoot(t)
	docRaw := extractDispositionExample(t, root, "docs/architecture/continuation-defect-ledger.md")

	var fromGo, fromDoc any
	if err := json.Unmarshal([]byte(dispositionSchemaExample), &fromGo); err != nil {
		t.Fatalf("dispositionSchemaExample is not valid JSON: %v\n%s", err, dispositionSchemaExample)
	}
	if err := json.Unmarshal([]byte(docRaw), &fromDoc); err != nil {
		t.Fatalf("the architecture doc's example is not valid JSON: %v\n%s", err, docRaw)
	}
	if !reflect.DeepEqual(fromGo, fromDoc) {
		t.Errorf("the schema echoed inline on rejection must be the same document the architecture doc tells agents to copy; they have drifted.\ngo:\n%s\ndoc:\n%s", dispositionSchemaExample, docRaw)
	}
}

// TestDispositionSchemaExampleIsAcceptedByProductionReader — the inline hint is
// itself a legal file. An example that the gate would reject teaches the next
// dispatch to fail again.
func TestDispositionSchemaExampleIsAcceptedByProductionReader(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, dispositionFile), []byte(dispositionSchemaExample), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	claims, diags, blocked := readDispositions(ws, 1398)
	if blocked {
		t.Fatalf("the inline schema hint must be a document the production reader accepts. diagnostics:\n%s", diagsText(diags))
	}
	if len(claims) != 2 {
		t.Fatalf("expected the hint to parse into 2 claims, got %d", len(claims))
	}
}
