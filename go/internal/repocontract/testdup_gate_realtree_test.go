package repocontract_test

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
			t.Fatal("no go.mod above the repocontract directory")
		}
		dir = parent
	}
}

func TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline(t *testing.T) {
	root := repositoryRoot(t)
	for _, e := range gateregistry.Entries() {
		if e.Name != "testdup-ratchet" {
			continue
		}
		verdict, err := e.Run(context.Background(), gatecontract.Input{Root: root}, gatecontract.Ports{})
		if err != nil {
			t.Fatal(err)
		}
		for _, red := range verdict.Reds {
			t.Error(red)
		}
		return
	}
	t.Fatal("the registry has no testdup-ratchet entry")
}
