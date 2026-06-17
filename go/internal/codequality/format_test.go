package codequality

import (
	"os"
	"path/filepath"
	"testing"
)

const dirtyGo = "package sample\n\nfunc Sample() int {\n\tx :=  1\n\treturn x\n}\n"
const cleanGo = "package sample\n\nfunc Sample() int {\n\tx := 1\n\treturn x\n}\n"

// FormatGoFiles must rewrite a gofmt-dirty file to clean and report it as fixed,
// so a builder's formatting lapse never reaches the audit gofmt gate (I9).
func TestFormatGoFiles_ReformatsDirtyFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(fp, []byte(dirtyGo), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("UnformattedGoFiles (setup): %v", err)
	}
	if len(before) == 0 {
		t.Fatal("setup: expected sample.go to be gofmt-dirty")
	}

	fixed, err := FormatGoFiles(dir)
	if err != nil {
		t.Fatalf("FormatGoFiles: %v", err)
	}
	if len(fixed) != 1 || filepath.Base(fixed[0]) != "sample.go" {
		t.Fatalf("want sample.go reformatted, got %v", fixed)
	}

	after, err := UnformattedGoFiles(dir)
	if err != nil {
		t.Fatalf("UnformattedGoFiles (after): %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("dir still dirty after FormatGoFiles: %v", after)
	}
	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("ReadFile after FormatGoFiles: %v", err)
	}
	if string(got) != cleanGo {
		t.Fatalf("file not gofmt-clean after FormatGoFiles:\n%q", string(got))
	}
}

// A clean dir is a no-op: nothing reformatted, no error — so the post-build step
// is byte-identical for a well-behaved builder.
func TestFormatGoFiles_NoOpOnCleanDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(cleanGo), 0o644); err != nil {
		t.Fatal(err)
	}
	fixed, err := FormatGoFiles(dir)
	if err != nil {
		t.Fatalf("FormatGoFiles: %v", err)
	}
	if len(fixed) != 0 {
		t.Fatalf("want no reformat on a clean dir, got %v", fixed)
	}
}

// ModuleDir resolves the go/ submodule when present and falls back to root —
// the single source the audit gate and the post-build normalizer both consume.
func TestModuleDir_PrefersGoSubdir(t *testing.T) {
	// Empty root is the load-bearing guard: "" must stay "" so FormatGoFiles
	// never hands gofmt an empty dir arg (which would scan the cwd).
	if got := ModuleDir(""); got != "" {
		t.Fatalf("empty root: want %q, got %q", "", got)
	}
	root := t.TempDir()
	if got := ModuleDir(root); got != root {
		t.Fatalf("no go/ subdir: want %q, got %q", root, got)
	}
	goDir := filepath.Join(root, "go")
	if err := os.Mkdir(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ModuleDir(root); got != goDir {
		t.Fatalf("with go/ subdir: want %q, got %q", goDir, got)
	}
}
