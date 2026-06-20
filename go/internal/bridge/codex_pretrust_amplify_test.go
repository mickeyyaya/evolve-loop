// codex_pretrust_amplify_test.go — Cycle-1 test-amplification adversarial tests.
//
// These probe invariants orthogonal to the TDD 2-goroutine regression guard:
//   - 3-goroutine concurrent pretrust (all entries survive, not just 2)
//   - Pre-seeded file preservation under concurrent writes
//   - Same-path idempotency under concurrent writes (no duplicate sections)
//   - High-stress 10-goroutine scenario under -race
//
// Anti-bias: written from the specification only; implementation not read.
// Run with -race to exercise the flock.WithPathLock serialization path.

package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestPretrustCodexProjects_Concurrent_ThreeGoroutines_AllSurvive verifies
// that a third goroutine's entry is not dropped under the 2-goroutine flock
// pattern. The 2-goroutine test guards against the original lost-update race;
// this test guards against a hypothetical "second writer wins, third ignored"
// regression that a 2-goroutine test would miss.
func TestPretrustCodexProjects_Concurrent_ThreeGoroutines_AllSurvive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfgs := []*Config{
		{Worktree: "/amp/wt-alpha", codexConfigPath: path},
		{Worktree: "/amp/wt-beta", codexConfigPath: path},
		{Worktree: "/amp/wt-gamma", codexConfigPath: path},
	}

	var wantHeaders []string
	for _, c := range cfgs {
		wantHeaders = append(wantHeaders, codexProjectHeader(c.Worktree))
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, len(cfgs))

	for i, c := range cfgs {
		wg.Add(1)
		go func(idx int, cfg *Config) {
			defer wg.Done()
			<-start
			errs[idx] = pretrustCodexProjects(cfg)
		}(i, c)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: pretrustCodexProjects: %v", i, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(got)

	for _, h := range wantHeaders {
		if !strings.Contains(content, h) {
			t.Errorf("entry %q missing from final file:\n%s", h, content)
		}
	}
	// Guard against duplicated sections (another failure mode).
	for _, h := range wantHeaders {
		if n := strings.Count(content, h); n != 1 {
			t.Errorf("entry %q appears %d times (want 1):\n%s", h, n, content)
		}
	}
}

// TestPretrustCodexProjects_Concurrent_PreSeededFileSurvives asserts that
// a pre-existing trust entry is not evicted when two goroutines concurrently
// add new entries. A lost-update regression in the read-modify-write cycle
// would silently drop the pre-seeded entry.
func TestPretrustCodexProjects_Concurrent_PreSeededFileSurvives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Seed a single entry first, outside the concurrent section.
	seed := &Config{Worktree: "/amp/wt-seed", codexConfigPath: path}
	if err := pretrustCodexProjects(seed); err != nil {
		t.Fatalf("seed pretrustCodexProjects: %v", err)
	}
	seedHeader := codexProjectHeader(seed.Worktree)

	// Now concurrently add two more entries.
	concurrent := []*Config{
		{Worktree: "/amp/wt-concurrent-A", codexConfigPath: path},
		{Worktree: "/amp/wt-concurrent-B", codexConfigPath: path},
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, len(concurrent))

	for i, c := range concurrent {
		wg.Add(1)
		go func(idx int, cfg *Config) {
			defer wg.Done()
			<-start
			errs[idx] = pretrustCodexProjects(cfg)
		}(i, c)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent goroutine %d: pretrustCodexProjects: %v", i, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(got)

	// Original seeded entry must survive.
	if !strings.Contains(content, seedHeader) {
		t.Errorf("pre-seeded entry %q lost from file after concurrent writes:\n%s", seedHeader, content)
	}
	// Both concurrent entries must also survive.
	for _, c := range concurrent {
		h := codexProjectHeader(c.Worktree)
		if !strings.Contains(content, h) {
			t.Errorf("concurrent entry %q missing from final file:\n%s", h, content)
		}
	}
}

// TestPretrustCodexProjects_Concurrent_SamePath_NoDuplicateSection verifies
// idempotency under contention: two goroutines pretrustCodexProjects the
// SAME worktree path. The flock-protected RMW must produce exactly ONE
// [projects."<path>"] section — not zero (lost-update) and not two (double-write).
func TestPretrustCodexProjects_Concurrent_SamePath_NoDuplicateSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	shared := &Config{Worktree: "/amp/wt-shared-path", codexConfigPath: path}
	header := codexProjectHeader(shared.Worktree)

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = pretrustCodexProjects(shared)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: pretrustCodexProjects: %v", i, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(got)

	n := strings.Count(content, header)
	if n != 1 {
		t.Errorf("same-path concurrent pretrust: got %d occurrence(s) of %q, want exactly 1:\n%s", n, header, content)
	}
}

// TestPretrustCodexProjects_Concurrent_HighStress_TenGoroutines runs 10
// goroutines with distinct worktree paths under a start-barrier, then asserts
// all 10 entries are present in the final file exactly once. This stresses
// the flock serialization queue beyond the 2- and 3-goroutine scenarios and
// exercises the OS file-lock fairness properties.
func TestPretrustCodexProjects_Concurrent_HighStress_TenGoroutines(t *testing.T) {
	const n = 10
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfgs := make([]*Config, n)
	for i := range n {
		cfgs[i] = &Config{Worktree: fmt.Sprintf("/amp/stress/wt-%02d", i), codexConfigPath: path}
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i, c := range cfgs {
		wg.Add(1)
		go func(idx int, cfg *Config) {
			defer wg.Done()
			<-start
			errs[idx] = pretrustCodexProjects(cfg)
		}(i, c)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: pretrustCodexProjects: %v", i, err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(got)

	for _, c := range cfgs {
		h := codexProjectHeader(c.Worktree)
		if count := strings.Count(content, h); count != 1 {
			t.Errorf("entry %q: got %d occurrence(s), want exactly 1", h, count)
		}
	}
}

// TestPretrustCodexProjects_Concurrent_MultiRoundTwoGoroutines runs the
// 2-goroutine scenario for 20 rounds, each with a fresh temp file, amplifying
// the probability of exposing a non-deterministic lost-update that the
// original 5-count single-file run might miss in favorable scheduling windows.
func TestPretrustCodexProjects_Concurrent_MultiRoundTwoGoroutines(t *testing.T) {
	const rounds = 20
	for round := range rounds {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")

		cfgA := &Config{Worktree: fmt.Sprintf("/amp/round-%02d/wt-A", round), codexConfigPath: path}
		cfgB := &Config{Worktree: fmt.Sprintf("/amp/round-%02d/wt-B", round), codexConfigPath: path}
		hA := codexProjectHeader(cfgA.Worktree)
		hB := codexProjectHeader(cfgB.Worktree)

		start := make(chan struct{})
		var wg sync.WaitGroup
		errA, errB := error(nil), error(nil)

		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			errA = pretrustCodexProjects(cfgA)
		}()
		go func() {
			defer wg.Done()
			<-start
			errB = pretrustCodexProjects(cfgB)
		}()
		close(start)
		wg.Wait()

		if errA != nil {
			t.Fatalf("round %d goroutine A: %v", round, errA)
		}
		if errB != nil {
			t.Fatalf("round %d goroutine B: %v", round, errB)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("round %d ReadFile: %v", round, err)
		}
		content := string(got)

		if !strings.Contains(content, hA) {
			t.Fatalf("round %d: entry A %q missing\n%s", round, hA, content)
		}
		if !strings.Contains(content, hB) {
			t.Fatalf("round %d: entry B %q missing\n%s", round, hB, content)
		}
	}
}
