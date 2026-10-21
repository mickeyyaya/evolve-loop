package tokenusage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasCollector_OnlyTheFamiliesWithTheirOwnCollector(t *testing.T) {
	t.Parallel()
	for driver, want := range map[string]bool{
		"": true, "claude": true, "claude-tmux": true, "CLAUDE-TMUX": true, "claude-headless": true,
		"agy-tmux": true, "agy-claude-tmux": true, "AGY": true,
		"codex-tmux": false, "ollama-tmux": false, "my-claude-wrapper": false, "claudex": false, "agyx": false,
	} {
		if got := HasCollector(driver); got != want {
			t.Errorf("HasCollector(%q) = %t, want %t", driver, got, want)
		}
	}
}

func TestResolver_ADriverWithNoCollectorSaysWhyItIsUnmeasured(t *testing.T) {
	t.Parallel()
	resolve := Resolver(Roots{}, nil)

	codex, err := resolve(Window{Driver: "codex-tmux"})
	if err != nil || codex.Source != SourceNone || !strings.Contains(codex.Warn, `driver "codex-tmux" has no usage collector`) {
		t.Fatalf("codex = %+v, %v; want source none with the no-collector reason", codex, err)
	}
	agy, err := resolve(Window{Driver: "agy-tmux"})
	if err != nil || agy.Source != SourceNone || !strings.Contains(agy.Warn, "token usage uncovered") || strings.Contains(agy.Warn, "no usage collector") {
		t.Fatalf("agy = %+v, %v; want the uncovered warning of a family whose collector found nothing", agy, err)
	}
}

func TestResolver_AnUppercaseClaudeDriverStillReadsItsTranscript(t *testing.T) {
	t.Parallel()
	worktree := "/repo/worktrees/cycle-623"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-623")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-08T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-08T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":200,"output_tokens":40,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "sess1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := DefaultResolver(root)(Window{Driver: "CLAUDE-TMUX", Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)})

	if err != nil || res.Source != SourceTranscript || EffectiveWindow("CLAUDE-TMUX") != claudeEffectiveWindow {
		t.Fatalf("CLAUDE-TMUX = %+v, %v; HasCollector says claude in any case, so the chain must read the transcript too", res, err)
	}
}

func TestEffectiveWindow_EveryCollectorFamilyInAnyCaseHasAWindow(t *testing.T) {
	t.Parallel()
	for _, driver := range []string{"agy-claude-tmux", "AGY-tmux", "Claude-Tmux", "CODEX-TMUX", "Codex"} {
		if got := EffectiveWindow(driver); got != claudeEffectiveWindow {
			t.Errorf("EffectiveWindow(%q) = %d, want %d: a driver that HasCollector or a measured codex driver must not read fill_pct -1", driver, got, claudeEffectiveWindow)
		}
	}
	for driver := range map[string]bool{"agy-claude-tmux": true, "AGY-tmux": true, "Claude-Tmux": true} {
		if !HasCollector(driver) {
			t.Errorf("HasCollector(%q) = false; the window and the collector come from one family table", driver)
		}
	}
}
