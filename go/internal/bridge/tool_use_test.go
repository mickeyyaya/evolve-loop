package bridge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestHasToolUse_FollowsTheDriverManifest(t *testing.T) {
	t.Parallel()
	for cli, want := range map[string]bool{
		"ollama-tmux": false,
		"claude-tmux": true,
		"codex-tmux":  true,
		"agy-tmux":    true,
		"nope-tmux":   false,
	} {
		if got := HasToolUse(cli); got != want {
			t.Errorf("HasToolUse(%q) = %v, want %v", cli, got, want)
		}
	}
}

func TestHasToolUse_EveryRegisteredDriverLoadsItsManifest(t *testing.T) {
	t.Parallel()
	for _, driver := range llmroute.Drivers() {
		toolless := driver == "ollama-tmux"
		if got := HasToolUse(driver); got != !toolless {
			t.Errorf("HasToolUse(%q) = %v, want %v: the manifest of a registered driver must load", driver, got, !toolless)
		}
		if got := LacksToolUse(driver); got != toolless {
			t.Errorf("LacksToolUse(%q) = %v, want %v", driver, got, toolless)
		}
	}
}

func TestLacksToolUse_AnUnloadableManifestIsNoEvidenceOfNoToolUse(t *testing.T) {
	dir := t.TempDir()
	useBridgeManifestDir(t, dir)
	for name, body := range map[string]string{
		"claude-tmux.json": `{`,
		"agy-tmux.json":    `{"base":"no-such-manifest"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		cli       string
		hasTools  bool
		lacksTool bool
	}{
		{"claude-tmux", false, false},
		{"agy-tmux", false, false},
		{"codex-tmux", true, false},
		{"ollama-tmux", false, true},
		{"nope-tmux", false, false},
	} {
		if got := HasToolUse(tc.cli); got != tc.hasTools {
			t.Errorf("HasToolUse(%q) = %v, want %v", tc.cli, got, tc.hasTools)
		}
		if got := LacksToolUse(tc.cli); got != tc.lacksTool {
			t.Errorf("LacksToolUse(%q) = %v, want %v: only a manifest that says toolless is evidence", tc.cli, got, tc.lacksTool)
		}
	}
}
