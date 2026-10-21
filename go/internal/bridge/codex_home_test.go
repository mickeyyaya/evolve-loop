package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexHome_HonorsCODEXHOMELikeCodexDoes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	if got, err := defaultCodexConfigPath(); err != nil || got != filepath.Join(dir, "config.toml") {
		t.Fatalf("config path = %q (%v), want %s/config.toml", got, err, dir)
	}
	if got, err := defaultCodexVersionPath(); err != nil || got != filepath.Join(dir, "version.json") {
		t.Fatalf("version path = %q (%v), want %s/version.json", got, err, dir)
	}
	t.Setenv("CODEX_HOME", "")
	home, _ := os.UserHomeDir()
	if got, _ := defaultCodexConfigPath(); got != filepath.Join(home, ".codex", "config.toml") {
		t.Errorf("without CODEX_HOME the path = %q, want the home default", got)
	}
	t.Setenv("HOME", "")
	if got, err := defaultCodexVersionPath(); err == nil || got != "" {
		t.Errorf("with no home at all: version path = %q, err %v; want an error and no path", got, err)
	}
	if got, err := defaultCodexConfigPath(); err == nil || got != "" {
		t.Errorf("with no home at all: config path = %q, err %v; want an error and no path", got, err)
	}
}

func TestPruneDeadCodexProjects_DropsOnlyDeadTempAndWorktreeEntries(t *testing.T) {
	t.Parallel()
	live := t.TempDir()
	deadTemp := filepath.Join(os.TempDir(), "evolve-models-probe-gone-1234")
	deadWorktree := "/repo/.evolve/worktrees/cycle-12-gone"
	deadUser := "/Users/someone/project-moved"
	deadOtherTemp := filepath.Join(os.TempDir(), "someones-scratch-gone")
	config := "model = \"gpt\"\n\n" +
		codexProjectHeader(live) + "\ntrust_level = \"trusted\"\n\n" +
		codexProjectHeader(deadTemp) + "\ntrust_level = \"trusted\"\n\n" +
		codexProjectHeader(deadWorktree) + "\ntrust_level = \"trusted\"\n\n" +
		codexProjectHeader(deadUser) + "\ntrust_level = \"trusted\"\n\n" +
		codexProjectHeader(deadOtherTemp) + "\ntrust_level = \"trusted\"\n\n" +
		codexProjectHeader(filepath.Join(os.TempDir(), "TestSomething123", "001")) + "\ntrust_level = \"trusted\"\n\n" +
		"[projects.\"bad\\q\"]\ntrust_level = \"trusted\"\n\n" +
		"[notice]\nhide_rate_limit_model_nudge = true\n"

	got := pruneDeadCodexProjects(config, nil)

	for _, keep := range []string{"model = \"gpt\"", codexProjectHeader(live), codexProjectHeader(deadUser), codexProjectHeader(deadOtherTemp), "[projects.\"bad\\q\"]", "[notice]"} {
		if !strings.Contains(got, keep) {
			t.Errorf("pruned config lost %q:\n%s", keep, got)
		}
	}
	for _, drop := range []string{codexProjectHeader(deadTemp), codexProjectHeader(deadWorktree), "TestSomething123"} {
		if strings.Contains(got, drop) {
			t.Errorf("pruned config keeps the dead entry %q:\n%s", drop, got)
		}
	}
	if pruneDeadCodexProjects(got, nil) != got {
		t.Error("pruning twice must change nothing")
	}
}

func TestPretrustCodexProjects_PrunesTheDeadEntriesItRewrites(t *testing.T) {
	t.Parallel()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	dead := filepath.Join(os.TempDir(), "evolve-models-probe-gone-5678")
	if err := os.WriteFile(configPath, []byte(codexProjectHeader(dead)+"\ntrust_level = \"trusted\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()

	if err := pretrustCodexProjects(&Config{Worktree: worktree, codexConfigPath: configPath}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(configPath)
	if strings.Contains(string(raw), dead) || !strings.Contains(string(raw), codexProjectHeader(worktree)) {
		t.Fatalf("config = %q: the rewrite must drop the dead temp entry and add the live worktree", raw)
	}
}

func TestPruneDeadCodexProjects_KeepsAPathTheCallTrusts(t *testing.T) {
	t.Parallel()
	pending := "/repo/.evolve/worktrees/cycle-99-new"
	config := codexProjectHeader(pending) + "\ntrust_level = \"trusted\"\n"
	if got := pruneDeadCodexProjects(config, []string{pending}); got != config {
		t.Fatalf("pruned = %q: the worktree this launch trusts may not exist yet, so it stays", got)
	}
}

func TestPruneDeadCodexProjects_FailsSafeOnHeadersItDoesNotKnow(t *testing.T) {
	t.Parallel()
	dead := filepath.Join(os.TempDir(), "evolve-models-probe-gone-91")
	config := codexProjectHeader(dead) + "\ntrust_level = \"trusted\"\n" +
		"  [notice]\nhide_rate_limit_model_nudge = true\n" +
		"[projects.\"" + dead + "\".sub]\nx = 1\n" +
		codexProjectHeader(dead) + " # a comment\ntrust_level = \"trusted\"\n"

	got := pruneDeadCodexProjects(config, nil)

	for _, keep := range []string{"  [notice]", "hide_rate_limit_model_nudge = true", ".sub]", "# a comment"} {
		if !strings.Contains(got, keep) {
			t.Errorf("pruned config lost %q: an indented header ends the dead table, and an unknown header form is kept:\n%s", keep, got)
		}
	}
}
