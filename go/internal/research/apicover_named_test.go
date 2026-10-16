package research

import (
	"context"
	"testing"
)

func TestFileKB_SatisfiesKBInterface(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "audit.yaml", auditEgpsLesson)

	var kb KB = NewFileKB([]string{dir})

	got, err := kb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("Lookup via KB interface: %v", err)
	}
	if len(got) != 1 || got[0].ID != "inst-L001" {
		t.Fatalf("Lookup through KB interface returned %v, want [inst-L001]", ids(got))
	}
}

func TestFileKB_RootsFieldBound(t *testing.T) {
	var fkb *FileKB = NewFileKB(nil)
	got, err := fkb.Lookup(context.Background(), Query{Source: "audit"})
	if err != nil {
		t.Fatalf("empty-roots FileKB should not error (novel-failure signal), got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty-roots FileKB returned %v, want no matches", ids(got))
	}
}
