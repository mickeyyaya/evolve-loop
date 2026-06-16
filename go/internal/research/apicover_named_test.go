package research

import (
	"context"
	"testing"
)

// TestFileKB_SatisfiesKBInterface names both FileKB (the concrete type) and the
// KB read-port interface, pinning the contract the orchestrator depends on:
// *FileKB must implement research.KB (core/orchestrator.go holds a `kb research.KB`
// field populated by NewFileKB). The interface binding proves satisfaction; the
// runtime Lookup call through the interface value proves it dispatches.
func TestFileKB_SatisfiesKBInterface(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "audit.yaml", auditEgpsLesson)

	// Bind the concrete *FileKB to the KB interface — this is exactly how the
	// composition root wires it (var kb research.KB = NewFileKB(...)).
	var kb KB = NewFileKB([]string{dir})

	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("Lookup via KB interface: %v", err)
	}
	if len(got) != 1 || got[0].ID != "inst-L001" {
		t.Fatalf("Lookup through KB interface returned %v, want [inst-L001]", ids(got))
	}
}

// TestFileKB_RootsFieldBound names the FileKB type via a typed nil-roots
// construction and pins the documented contract that a corpus-free FileKB
// returns the novel-failure signal (empty, no error) rather than panicking.
func TestFileKB_RootsFieldBound(t *testing.T) {
	var fkb *FileKB = NewFileKB(nil) // no roots
	got, err := fkb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("empty-roots FileKB should not error (novel-failure signal), got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty-roots FileKB returned %v, want no matches", ids(got))
	}
}
