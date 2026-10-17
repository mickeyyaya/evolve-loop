package structure

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	trailAdapterDir = "internal/adapters/codeindex/trail"
	trailBinToken   = ".bin/trail"
	sysexecImport   = "github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func intersection(a, b []string) []string {
	var both []string
	for _, x := range a {
		if slices.Contains(b, x) {
			both = append(both, x)
		}
	}
	return both
}

func TestOnlyTheTrailAdapterRunsTheTrailBinary(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	literal, err := LiteralFiles(root, trailBinToken, ".")
	if err != nil {
		t.Fatal(err)
	}
	runners, err := ExecImporters(root, ".")
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range intersection(literal, runners) {
		if !strings.HasPrefix(filepath.ToSlash(file), trailAdapterDir+"/") {
			t.Errorf("%s runs a process and names the trail binary; only %s may run it", file, trailAdapterDir)
		}
	}
}

func TestTheTrailAdapterIsTheHomeThatRunsAProcess(t *testing.T) {
	t.Parallel()

	runners, err := ExecImporters(filepath.Join("..", ".."), trailAdapterDir)

	if err != nil || len(runners) == 0 {
		t.Errorf("ExecImporters = %v, %v; want the adapter to import a process runner", runners, err)
	}
}

func TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")

	runners, err := ExecImporters(root, "internal/codeindex")
	named, namedErr := LiteralFiles(root, "trail", "internal/codeindex")

	if err != nil || namedErr != nil || len(runners) != 0 || len(named) != 0 {
		t.Errorf("runners = %v (%v), files naming trail = %v (%v); want none", runners, err, named, namedErr)
	}
}

func binaryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a/run.go":      "package a\nimport \"os/exec\"\nvar _ = exec.Command\nconst bin = \"x/node_modules/.bin/trail\"\n",
		"a/sys.go":      "package a\nimport \"" + sysexecImport + "\"\nvar _ = sysexec.Command\n",
		"b/literal.go":  "package b\nconst bin = \"x/.bin/trail\"\n",
		"b/clean.go":    "package b\nconst n = 1\n",
		"b/run_test.go": "package b\nimport \"os/exec\"\nvar _ = exec.Command\nconst bin = \".bin/trail\"\n",
		"c/deep/d.go":   "package d\nimport \"os/exec\"\nvar _ = exec.Command\n",
	})
	return root
}

func TestExecImporters_ListsNonTestFilesThatImportAProcessRunner(t *testing.T) {
	t.Parallel()

	got, err := ExecImporters(binaryFixture(t), ".")

	want := []string{filepath.Join("a", "run.go"), filepath.Join("a", "sys.go"), filepath.Join("c", "deep", "d.go")}
	slices.Sort(got)
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("ExecImporters = %v, %v, want %v", got, err, want)
	}
}

func TestExecImporters_NarrowsToTheGivenDirectories(t *testing.T) {
	t.Parallel()

	got, err := ExecImporters(binaryFixture(t), "c")

	if err != nil || !slices.Equal(got, []string{filepath.Join("c", "deep", "d.go")}) {
		t.Errorf("ExecImporters = %v, %v", got, err)
	}
}

func TestLiteralFiles_ListsNonTestFilesWithAStringLiteralHoldingTheToken(t *testing.T) {
	t.Parallel()

	got, err := LiteralFiles(binaryFixture(t), trailBinToken, ".")

	want := []string{filepath.Join("a", "run.go"), filepath.Join("b", "literal.go")}
	slices.Sort(got)
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("LiteralFiles = %v, %v, want %v", got, err, want)
	}
}

func TestBinaryScans_AnAbsentDirectoryIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, execErr := ExecImporters(root, "gone")
	_, literalErr := LiteralFiles(root, "x", "gone")

	if execErr == nil || literalErr == nil {
		t.Errorf("errors = %v, %v; want both", execErr, literalErr)
	}
}

func TestBinaryScans_AnUnparsableFileIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a/broken.go": "package a\nfunc ("})

	_, execErr := ExecImporters(root, ".")
	_, literalErr := LiteralFiles(root, "x", ".")

	if execErr == nil || literalErr == nil {
		t.Errorf("errors = %v, %v; want both", execErr, literalErr)
	}
}
