package emptytests_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the emptytests directory")
		}
		dir = parent
	}
}

func TestTree_HasNoNewEmptyTest(t *testing.T) {
	root := repositoryRoot(t)
	for _, e := range gateregistry.Entries() {
		if e.Name != "empty-tests" {
			continue
		}
		verdict, err := e.Run(context.Background(), gatecontract.Input{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		for _, red := range verdict.Reds {
			t.Errorf("the tree holds a Go test with an empty body; give it a statement or t.Skip with a reason:\n%s", red)
		}
		return
	}
	t.Fatal("the registry has no empty-tests entry")
}
