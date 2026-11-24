package ship

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRepoContractPack_RejectsALaneThatAddsAnEmptyTest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	module := filepath.Join(root, "go")
	copyEmptytestsPackage(t, filepath.Join(module, "internal", "emptytests"))
	writeScratch(t, filepath.Join(module, "internal", "emptytests", "scratch_tree_test.go"), scratchTreeTest)
	writeScratch(t, filepath.Join(module, "go.mod"), "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	writeScratch(t, filepath.Join(module, "internal", "lane", "lane_test.go"), "package lane\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n")
	dirs := slices.Clone(repoContractPackages)
	for _, selection := range repoContractSelections {
		dirs = append(dirs, selection.Package)
	}
	for _, pattern := range dirs {
		dir := filepath.Join(module, filepath.FromSlash(strings.TrimSuffix(pattern, "/...")))
		if _, err := os.Stat(dir); err != nil {
			writeScratch(t, filepath.Join(dir, "doc.go"), "package "+filepath.Base(dir)+"\n")
		}
	}
	reds, diagnostic, err := RunRepoContractPack(context.Background(), "", root, nil)
	if err == nil || len(reds) == 0 {
		t.Fatalf("the pack must go red for a lane that adds an empty test; got reds=%v diagnostic=%q err=%v", reds, diagnostic, err)
	}
	if !strings.Contains(strings.Join(reds, " "), "emptytests") || !strings.Contains(diagnostic, "lane_test.go:5: TestX has an empty body") {
		t.Fatalf("the red must name the emptytests suite and the offender; got reds=%v diagnostic=%q", reds, diagnostic)
	}
}

func copyEmptytestsPackage(t *testing.T, dst string) {
	t.Helper()
	src := filepath.Join("..", "..", "emptytests")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "gate_realtree_test.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		writeScratch(t, filepath.Join(dst, e.Name()), string(data))
	}
}

func writeScratch(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const scratchTreeTest = `package emptytests_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/emptytests"
)

func TestTree_HasNoNewEmptyTest(t *testing.T) {
	found, err := emptytests.Scan(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := emptytests.Check(found); err != nil {
		t.Error(err)
	}
}
`
