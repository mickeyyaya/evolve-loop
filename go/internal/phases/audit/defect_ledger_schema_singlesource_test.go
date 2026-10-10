package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
