package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestPretrustCodexProjects_ConcurrentTwoGoroutines is the Slice-2 regression
// test for ADR-0049 N10 (concurrency-arch-slices campaign). Two goroutines each
// pretrust a DISTINCT worktree path against ONE shared EVOLVE_CODEX_CONFIG_PATH
// temp file concurrently, then assert the final TOML contains BOTH
// [projects."..."] entries.
//
// Pre-fix behaviour: read-merge-write-RENAME was last-writer-wins — two
// goroutines that each read the empty initial state before either writes
// overwrite each other's output, leaving only the last writer's entry.
// Post-fix: flock.WithPathLock(configPath) at codex_pretrust.go:79 serializes
// the whole RMW so every append composes losslessly and no entry is dropped.
//
// Run with -race to surface unprotected shared state; the start barrier forces
// both goroutines to read the same (empty) initial state simultaneously,
// maximising contention. -count=5 (eval grader) repeats the race 5 times to
// prevent a lucky interleaving from hiding a regression.
func TestPretrustCodexProjects_ConcurrentTwoGoroutines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", path)

	cfgA := &Config{Worktree: "/tmp/pretrust-conc-wt-A"}
	cfgB := &Config{Worktree: "/tmp/pretrust-conc-wt-B"}

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		errs[0] = pretrustCodexProjects(cfgA)
	}()
	go func() {
		defer wg.Done()
		<-start
		errs[1] = pretrustCodexProjects(cfgB)
	}()

	close(start) // release both goroutines simultaneously
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: pretrustCodexProjects: %v", i, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read merged config: %v", err)
	}
	gotStr := string(got)

	for _, h := range []string{
		codexProjectHeader(cfgA.Worktree),
		codexProjectHeader(cfgB.Worktree),
	} {
		if !strings.Contains(gotStr, h) {
			t.Errorf("lost trust entry under concurrent pretrust: %q absent\n%s", h, gotStr)
		}
	}
}
