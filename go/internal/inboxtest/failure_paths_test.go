package inboxtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

type fatalSentinel struct{}

type recorder struct {
	testing.TB
	dir    string
	failed bool
}

func (r *recorder) Helper()            {}
func (r *recorder) TempDir() string    { return r.dir }
func (r *recorder) Fatal(args ...any)  { r.failed = true; panic(fatalSentinel{}) }
func (r *recorder) Setenv(k, v string) {}

func failsOn(t *testing.T, dir string, fn func(tb testing.TB)) (failed bool) {
	t.Helper()
	r := &recorder{dir: dir}
	defer func() {
		if p := recover(); p != nil {
			if _, ok := p.(fatalSentinel); !ok {
				panic(p)
			}
			failed = r.failed
		}
	}()
	fn(r)
	return false
}

func fileAt(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plain-file")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFixtures_EachHelperFailsTheTestWhenItsFilesystemStepFails(t *testing.T) {
	t.Parallel()
	blocker := fileAt(t)
	good := filepath.Join(t.TempDir(), "good.json")
	if err := os.WriteFile(good, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "absent.json")
	brokenRegistry := t.TempDir()
	if err := os.MkdirAll(filepath.Join(brokenRegistry, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(brokenRegistry), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(tb testing.TB){
		"EmptyRoot under a file":         func(tb testing.TB) { EmptyRoot(tb) },
		"WriteFile parent is a file":     func(tb testing.TB) { WriteFile(tb, filepath.Join(blocker, "x", "f"), "b") },
		"WriteFile target is a dir":      func(tb testing.TB) { WriteFile(tb, t.TempDir(), "b") },
		"ItemFields missing":             func(tb testing.TB) { ItemFields(tb, missing) },
		"ItemFields invalid json":        func(tb testing.TB) { ItemFields(tb, good) },
		"ReadRouted missing":             func(tb testing.TB) { ReadRouted(tb, missing) },
		"ReadRouted invalid json":        func(tb testing.TB) { ReadRouted(tb, good) },
		"TreeBytes missing root":         func(tb testing.TB) { TreeBytes(tb, missing) },
		"WriteLiveRunLease under a file": func(tb testing.TB) { WriteLiveRunLease(tb, blocker) },
		"WriteItem dir is a file":        func(tb testing.TB) { WriteItem(tb, blocker, "x") },
		"Bind under a file":              func(tb testing.TB) { Bind(tb, blocker, "x", continuation.Continuation{}) },
		"Bound registry is invalid json": func(tb testing.TB) { Bound(tb, brokenRegistry, "x") },
		"MkdirAll under a file":          func(tb testing.TB) { MkdirAll(tb, filepath.Join(blocker, "d")) },
		"RealDir missing":                func(tb testing.TB) { RealDir(tb, missing) },
	}
	for name, fn := range cases {
		dir := t.TempDir()
		if name == "EmptyRoot under a file" {
			dir = blocker
		}
		if !failsOn(t, dir, fn) {
			t.Errorf("%s: the helper did not fail the test", name)
		}
	}
}

func TestFixtures_WriteLiveRunLeaseFailsWhenTheLeaseCannotBeWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-1790", ".lease", "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !failsOn(t, t.TempDir(), func(tb testing.TB) { WriteLiveRunLease(tb, root) }) {
		t.Error("a lease path held by a directory must fail the test")
	}
}
