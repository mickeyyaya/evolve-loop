package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

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
	for _, h := range wantHeaders {
		if n := strings.Count(content, h); n != 1 {
			t.Errorf("entry %q appears %d times (want 1):\n%s", h, n, content)
		}
	}
}

func TestPretrustCodexProjects_Concurrent_PreSeededFileSurvives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	seed := &Config{Worktree: "/amp/wt-seed", codexConfigPath: path}
	if err := pretrustCodexProjects(seed); err != nil {
		t.Fatalf("seed pretrustCodexProjects: %v", err)
	}
	seedHeader := codexProjectHeader(seed.Worktree)

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

	if !strings.Contains(content, seedHeader) {
		t.Errorf("pre-seeded entry %q lost from file after concurrent writes:\n%s", seedHeader, content)
	}
	for _, c := range concurrent {
		h := codexProjectHeader(c.Worktree)
		if !strings.Contains(content, h) {
			t.Errorf("concurrent entry %q missing from final file:\n%s", h, content)
		}
	}
}

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
