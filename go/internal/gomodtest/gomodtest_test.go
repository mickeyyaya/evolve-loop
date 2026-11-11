package gomodtest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gomodtest"
)

func TestWriteModule_WritesEachFileUnderANewDirectoryAndCreatesParents(t *testing.T) {
	t.Parallel()

	dir := gomodtest.WriteModule(t, map[string]string{"go.mod": "module m\n", "a/b/c.go": "package b\n"})

	for name, want := range map[string]string{"go.mod": "module m\n", "a/b/c.go": "package b\n"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestWriteModule_GivesAnEmptyDirectoryForNoFiles(t *testing.T) {
	t.Parallel()

	dir := gomodtest.WriteModule(t, nil)

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Errorf("entries = %v, %v", entries, err)
	}
}

type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper()         {}
func (r *recorder) TempDir() string { return "/nonexistent-root/x" }
func (r *recorder) Fatal(a ...any)  { r.failed = true; panic(r) }

func TestWriteModule_FailsTheTestWhenTheDirectoryCannotBeMade(t *testing.T) {
	t.Parallel()
	r := &recorder{}

	func() {
		defer func() { _ = recover() }()
		gomodtest.WriteModule(r, map[string]string{"a.go": "x"})
	}()

	if !r.failed {
		t.Error("WriteModule did not fail on an unwritable directory")
	}
}

func TestWriteModule_FailsTheTestWhenAFileCannotBeWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	r := &recorder{}
	fixed := &fixedDir{recorder: r, dir: root}

	func() {
		defer func() { _ = recover() }()
		gomodtest.WriteModule(fixed, map[string]string{"a.go": "x"})
	}()

	if !r.failed {
		t.Error("WriteModule did not fail in a read-only directory")
	}
}

type fixedDir struct {
	*recorder
	dir string
}

func (f *fixedDir) TempDir() string { return f.dir }
