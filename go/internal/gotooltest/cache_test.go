package gotooltest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type fakeGo struct {
	mu    sync.Mutex
	calls [][]string
	envs  [][]string
	err   error
}

func (f *fakeGo) run(dir string, env []string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{dir}, args...))
	f.envs = append(f.envs, env)
	if f.err != nil {
		return []byte("compile output"), f.err
	}
	return nil, nil
}

func TestCacheBuild_RunsOneGoBuildForManyCallers(t *testing.T) {
	fake := &fakeGo{}
	cache := NewCache(fake.run)
	t.Cleanup(func() { _ = cache.Close() })
	spec := Spec{Dir: "/repo/go", Pkg: "./cmd/evolve", Tags: "evolve_test_phases", Name: "evolve"}
	paths := make([]string, 8)
	var wg sync.WaitGroup

	for i := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path, err := cache.Build(spec)
			if err != nil {
				t.Errorf("Build: %v", err)
			}
			paths[i] = path
		}()
	}
	wg.Wait()

	if len(fake.calls) != 1 {
		t.Fatalf("go invocations = %d (%v), want 1", len(fake.calls), fake.calls)
	}
	for _, path := range paths {
		if path != paths[0] || path == "" {
			t.Errorf("paths = %v, want one shared path", paths)
		}
	}
	want := []string{"/repo/go", "build", "-tags", "evolve_test_phases", "-o", paths[0], "./cmd/evolve"}
	if !slices.Equal(fake.calls[0], want) {
		t.Errorf("go call = %v, want %v", fake.calls[0], want)
	}
	if !slices.Contains(fake.envs[0], "GOMAXPROCS=2") {
		t.Errorf("the build env = %v, want the CPU limit", fake.envs[0])
	}
}

func TestCacheBuild_KeysByDirPackageTagsAndName(t *testing.T) {
	fake := &fakeGo{}
	cache := NewCache(fake.run)
	t.Cleanup(func() { _ = cache.Close() })
	specs := []Spec{
		{Dir: "/a", Pkg: "./cmd/x", Name: "x"},
		{Dir: "/b", Pkg: "./cmd/x", Name: "x"},
		{Dir: "/a", Pkg: "./cmd/y", Name: "x"},
		{Dir: "/a", Pkg: "./cmd/x", Tags: "t", Name: "x"},
		{Dir: "/a", Pkg: "./cmd/x", Name: "z"},
	}
	seen := map[string]bool{}

	for _, spec := range specs {
		path, err := cache.Build(spec)
		if err != nil {
			t.Fatalf("Build(%v): %v", spec, err)
		}
		seen[path] = true
	}

	if len(fake.calls) != len(specs) || len(seen) != len(specs) {
		t.Errorf("go calls = %d, distinct paths = %d, want %d of each", len(fake.calls), len(seen), len(specs))
	}
}

func TestCacheBuild_ABuildFailureReachesEveryCallerWithoutARetry(t *testing.T) {
	boom := errors.New("exit status 1")
	fake := &fakeGo{err: boom}
	cache := NewCache(fake.run)
	t.Cleanup(func() { _ = cache.Close() })
	spec := Spec{Dir: "/repo/go", Pkg: "./cmd/evolve", Name: "evolve"}

	for range 3 {
		path, err := cache.Build(spec)

		if path != "" || !errors.Is(err, boom) || !strings.Contains(err.Error(), "compile output") {
			t.Errorf("Build = (%q, %v), want the build error with the compiler output", path, err)
		}
	}
	if len(fake.calls) != 1 {
		t.Errorf("go invocations = %d, want 1", len(fake.calls))
	}
}

func TestCacheClose_RemovesTheBuildDirectoryAndIsRepeatable(t *testing.T) {
	cache := NewCache((&fakeGo{}).run)
	path, err := cache.Build(Spec{Dir: "/d", Pkg: "./p", Name: "bin"})
	if err != nil {
		t.Fatal(err)
	}
	root := path[:strings.LastIndex(path, "/")]
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the build directory must exist before Close: %v", err)
	}

	if err := cache.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("the build directory must be gone after Close, stat err = %v", err)
	}
	if err := cache.Close(); err != nil {
		t.Errorf("a second Close = %v, want nil", err)
	}
}

func TestExecRunner_RunsTheGoToolInTheDirectoryWithTheEnvironment(t *testing.T) {
	out, err := ExecRunner(t.TempDir(), Env(os.Environ()), "env", "GOFLAGS", "GOMAXPROCS")

	if err != nil {
		t.Fatalf("go env: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "-p=2") {
		t.Errorf("go env GOFLAGS GOMAXPROCS = %q, want -p=2 from the limited environment", out)
	}
}

func TestCacheBuild_ADirectoryFailureIsReturnedAndNoGoRunStarts(t *testing.T) {
	fake := &fakeGo{}
	cache := NewCache(fake.run)
	boom := errors.New("disk full")
	cache.mkdir = func(string, string) (string, error) { return "", boom }

	path, err := cache.Build(Spec{Dir: "/d", Pkg: "./p", Name: "bin"})

	if path != "" || !errors.Is(err, boom) || len(fake.calls) != 0 {
		t.Errorf("Build = (%q, %v) with %d go calls, want the directory error and no go run", path, err, len(fake.calls))
	}
}

func TestCacheBuild_KeepsTheBinaryNameAsTheFileName(t *testing.T) {
	cache := NewCache((&fakeGo{}).run)
	t.Cleanup(func() { _ = cache.Close() })

	path, err := cache.Build(Spec{Dir: "/d", Pkg: "./cmd/evolve-fake-cli", Name: "evolve-fake-cli"})

	if err != nil || filepath.Base(path) != "evolve-fake-cli" {
		t.Errorf("Build = (%q, %v), want a file named evolve-fake-cli", path, err)
	}
}

func TestCacheBuild_AnOutputDirectoryFailureIsReturnedAndNoGoRunStarts(t *testing.T) {
	fake := &fakeGo{}
	cache := NewCache(fake.run)
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cache.mkdir = func(string, string) (string, error) { return blocker, nil }

	path, err := cache.Build(Spec{Dir: "/d", Pkg: "./p", Name: "bin"})

	if path != "" || err == nil || len(fake.calls) != 0 {
		t.Errorf("Build = (%q, %v) with %d go calls, want an error and no go run", path, err, len(fake.calls))
	}
}
