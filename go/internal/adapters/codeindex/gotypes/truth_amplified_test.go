package gotypes_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/codeindex/gotypes"
)

func TestAmp_Truth_MissingIndexErrors(t *testing.T) {
	if _, err := gotypes.Truth(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("want error for missing index")
	}
}

func TestAmp_Truth_CorruptIndexErrors(t *testing.T) {
	p := filepath.Join(t.TempDir(), "index.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gotypes.Truth(p); err == nil {
		t.Fatal("want error for corrupt index")
	}
}

func TestAmp_Truth_EmptyFileErrorsOrEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "index.json")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := gotypes.Truth(p)
	if err == nil && len(got) != 0 {
		t.Fatalf("empty index gave %d symbols", len(got))
	}
}
