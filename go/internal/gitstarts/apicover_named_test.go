package gitstarts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAPICover_NamesEveryExportedSymbol(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s_test.go"), []byte("package s\nfunc s() { r.Git(\"status\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Count(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("Count = %d, want 1", got)
	}
	recorded := filepath.Join(t.TempDir(), "recorded.json")
	if err := os.WriteFile(recorded, []byte(`{"recorded": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, recorded); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}
