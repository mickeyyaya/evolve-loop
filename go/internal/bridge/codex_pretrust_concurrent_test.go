package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// cfg.codexConfigPath, not EVOLVE_CODEX_CONFIG_PATH, points both goroutines at one shared temp config, so the
// real ~/.codex is never touched. The start barrier makes both read the same empty file, the lost-update window.
func TestPretrustCodexProjects_ConcurrentTwoGoroutines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfgA := &Config{Worktree: "/tmp/pretrust-conc-wt-A", codexConfigPath: path}
	cfgB := &Config{Worktree: "/tmp/pretrust-conc-wt-B", codexConfigPath: path}

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
