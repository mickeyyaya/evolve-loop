package rawgitratchet_test

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
			t.Fatal("no go.mod above the rawgitratchet directory")
		}
		dir = parent
	}
}

func TestRatchet_NoNewRawGitFixtures(t *testing.T) {
	root := repositoryRoot(t)
	for _, e := range gateregistry.Entries() {
		if e.Name != "raw-git-ratchet" {
			continue
		}
		verdict, err := e.Run(context.Background(), gatecontract.Input{Root: root})
		for _, note := range verdict.Notes {
			t.Log(note)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, red := range verdict.Reds {
			t.Error(red)
		}
		return
	}
	t.Fatal("the registry has no raw-git-ratchet entry")
}
