package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
)

func writeRel(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runVerb(run func([]string, *bytes.Buffer, *bytes.Buffer) int, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func docsVerb(args []string, stdout, stderr *bytes.Buffer) int {
	return runDocs(args, nil, stdout, stderr)
}

func ciVerb(args []string, stdout, stderr *bytes.Buffer) int { return runCI(args, nil, stdout, stderr) }

func TestDocsIndex_CheckFailsOnAMissingIndexLineAndPassesAfterGenerate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRel(t, root, researchindex.IndexPath, "# Research Index\n")
	writeRel(t, root, researchindex.EntriesDir+"/2026-10-21-a.md", "## 2026-10-21 — a\n")

	code, _, stderr := runVerb(docsVerb, "index", "check", "--project-root", root)
	if code != exitRefused || !strings.Contains(stderr, "2026-10-21-a.md") {
		t.Fatalf("check = %d %q, want %d naming the entry", code, stderr, exitRefused)
	}
	if code, stdout, _ := runVerb(docsVerb, "index", "generate", "--project-root", root); code != 0 || !strings.Contains(stdout, "regenerated") {
		t.Fatalf("generate = %d %q", code, stdout)
	}
	if code, stdout, _ := runVerb(docsVerb, "index", "check", "--project-root", root); code != 0 || !strings.Contains(stdout, "in sync") {
		t.Fatalf("check after generate = %d %q", code, stdout)
	}
	if code, stdout, _ := runVerb(docsVerb, "index", "generate", "--project-root", root); code != 0 || !strings.Contains(stdout, "up to date") {
		t.Fatalf("a second generate = %d %q", code, stdout)
	}
}

func TestDocsIndex_ARealFaultIsAnIOExitAndABadVerbIsUsage(t *testing.T) {
	t.Parallel()
	if code, _, stderr := runVerb(docsVerb, "index", "check", "--project-root", t.TempDir()); code != exitIO || stderr == "" {
		t.Errorf("check of an empty tree = %d %q, want %d", code, stderr, exitIO)
	}
	if code, _, stderr := runVerb(docsVerb, "index", "generate", "--project-root", t.TempDir()); code != exitIO || stderr == "" {
		t.Errorf("generate of an empty tree = %d %q, want %d", code, stderr, exitIO)
	}
	for _, args := range [][]string{{"index"}, {"index", "write"}, {"index", "check", "extra"}, {"index", "check", "--bogus"}} {
		if code, _, stderr := runVerb(docsVerb, args...); code != exitUsage || !strings.Contains(stderr, "usage: evolve docs index") {
			t.Errorf("%v = %d %q, want usage", args, code, stderr)
		}
	}
}

func TestDocsIndex_TheProjectRootDefaultsToTheSourceRoot(t *testing.T) {
	root := t.TempDir()
	writeRel(t, root, researchindex.IndexPath, "# Research Index\n")
	writeRel(t, root, researchindex.EntriesDir+"/2026-10-21-a.md", "## 2026-10-21 — a\n")
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if code, _, _ := runVerb(docsVerb, "index", "generate"); code != 0 {
		t.Fatalf("generate = %d", code)
	}
	if body, _ := os.ReadFile(filepath.Join(root, researchindex.IndexPath)); !strings.Contains(string(body), researchindex.Begin) {
		t.Errorf("index = %q, want the generated region", body)
	}
}

func TestCIDarwinSet_ChecksAndWritesTheSetFileOfTheModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRel(t, root, "go/go.mod", "module m\n\ngo 1.23\n")
	writeRel(t, root, "go/mac/a_darwin.go", "package mac\n")

	if code, _, stderr := runVerb(ciVerb, "darwin-set", "check", "--project-root", root); code != exitRefused || !strings.Contains(stderr, "darwin-set generate") {
		t.Fatalf("check = %d %q", code, stderr)
	}
	if code, _, _ := runVerb(ciVerb, "darwin-set", "generate", "--project-root", root); code != 0 {
		t.Fatalf("generate = %d", code)
	}
	got, _ := os.ReadFile(filepath.Join(root, "go", ciparity.DarwinSetFile))
	if string(got) != ciparity.DarwinSetText([]string{"./mac"}) {
		t.Fatalf("set file = %q", got)
	}
	if code, _, _ := runVerb(ciVerb, "darwin-set", "check", "--project-root", root); code != 0 {
		t.Errorf("check after generate = %d", code)
	}
	if code, _, stderr := runVerb(ciVerb, "darwin-set", "frob"); code != exitUsage || !strings.Contains(stderr, "usage: evolve ci darwin-set") {
		t.Errorf("a bad verb = %d %q", code, stderr)
	}
}
