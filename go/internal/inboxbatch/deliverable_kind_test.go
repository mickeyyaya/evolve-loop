package inboxbatch

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFile_DeliverableKind — ADR-0099 slice 2: an inbox item declares the
// kind of thing it wants built; the harness projects it into the Task Contract
// block (the same single-source discipline as acceptance[]).
func TestLoadFile_DeliverableKind(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "netflix-margin.json")
	body := `{"id":"netflix-margin","title":"Netflix margin strategy","weight":0.8,"deliverable_kind":"document","acceptance":["two options"]}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, _, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if it.DeliverableKind != "document" {
		t.Errorf("DeliverableKind = %q, want document", it.DeliverableKind)
	}
}
