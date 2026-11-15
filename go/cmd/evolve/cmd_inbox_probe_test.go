package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestProbeTree_ReadsTheTestsAndThePathsOfTheProjectRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "p", "p_test.go"), []byte("package p\n\nfunc TestHere(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withProbes := inboxbatch.Item{Probes: []inboxbatch.Probe{{Kind: inboxbatch.ProbeTestExists, Arg: "TestHere"}}}

	tree, err := probeTree(root, withProbes)

	if err != nil || !tree.HasTest("TestHere") || tree.HasTest("TestGone") || !tree.PathExists("go/p/p_test.go") || tree.PathExists("go/p/gone.go") {
		t.Errorf("probeTree = %+v, %v; want the test index and the path check of the root", tree, err)
	}
}

func TestProbeTree_AnItemWithoutProbesNeverIndexesTheTests(t *testing.T) {
	t.Parallel()
	tree, err := probeTree(t.TempDir(), inboxbatch.Item{})

	if err != nil || tree.HasTest != nil || tree.PathExists == nil {
		t.Errorf("probeTree = %+v, %v; want only the path check", tree, err)
	}
}

func TestProbeTree_AnUnreadableTestFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "go", "x_test.go")); err != nil {
		t.Fatal(err)
	}

	_, err := probeTree(root, inboxbatch.Item{Probes: []inboxbatch.Probe{{Kind: inboxbatch.ProbeTestExists, Arg: "T"}}})

	if err == nil {
		t.Errorf("probeTree over an unreadable test file = nil, want the index error")
	}
}
