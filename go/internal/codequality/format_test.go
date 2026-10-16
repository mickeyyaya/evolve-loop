package codequality

import (
	"os"
	"path/filepath"
	"testing"
)

const dirtyGo = "package sample\n\nfunc Sample() int {\n\tx :=  1\n\treturn x\n}\n"
const cleanGo = "package sample\n\nfunc Sample() int {\n\tx := 1\n\treturn x\n}\n"

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

func TestModuleDir_PrefersGoSubdir(t *testing.T) {
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

func TestResolveModuleDir_PrefersNestedModule(t *testing.T) {
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	if err := os.Mkdir(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module example.com/nested\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveModuleDir(root)
	if err != nil {
		t.Fatalf("ResolveModuleDir: %v", err)
	}
	if got != goDir {
		t.Fatalf("ResolveModuleDir = %q, want nested module %q", got, goDir)
	}
}

func TestResolveModuleDir_IgnoresGoSupportDirectoryWithoutModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/root\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "go", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveModuleDir(root)
	if err != nil {
		t.Fatalf("ResolveModuleDir: %v", err)
	}
	if got != root {
		t.Fatalf("ResolveModuleDir = %q, want root module %q", got, root)
	}
}

func TestResolveModuleDir_ReportsGoSubdirLookupError(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("go", filepath.Join(root, "go")); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveModuleDir(root); err == nil {
		t.Fatal("ResolveModuleDir accepted a self-referential go/ symlink")
	}
}
