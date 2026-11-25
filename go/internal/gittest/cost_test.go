package gittest_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitstarts"
)

func TestEveryGitMethodHasCost(t *testing.T) {
	missing, err := gitstarts.MissingCosts(".")
	if err != nil {
		t.Fatalf("MissingCosts: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("exported gittest helpers that start git lack a StartCosts entry: %v", missing)
	}

	t.Run("flags uncosted exported helper", func(t *testing.T) {
		dir := t.TempDir()
		src := "package gittest\n\nimport \"os/exec\"\n\nfunc UncostedHelper(dir string) {\n\texec.Command(\"git\", \"-C\", dir, \"status\")\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "extra.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := gitstarts.MissingCosts(dir)
		if err != nil {
			t.Fatalf("MissingCosts: %v", err)
		}
		if !slices.Contains(got, "UncostedHelper") {
			t.Errorf("MissingCosts = %v, want UncostedHelper", got)
		}
	})
}
