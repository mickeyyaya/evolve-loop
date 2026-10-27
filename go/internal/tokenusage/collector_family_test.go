package tokenusage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasCollector_OnlyTheFormatsWithAnImplementation(t *testing.T) {
	t.Parallel()
	for format, want := range map[string]bool{
		FormatClaudeJSONL: true, FormatAgyConversation: true,
		"": false, "claude": false, "claude-tmux": false, "agy-tmux": false, "newfam-jsonl-v9": false,
	} {
		if got := HasCollector(format); got != want {
			t.Errorf("HasCollector(%q) = %t, want %t", format, got, want)
		}
	}
}

func TestResolver_ADriverWithNoCollectorIsNoCollectorNotZero(t *testing.T) {
	t.Parallel()
	resolve := Resolver(Roots{}, nil)

	codex, err := resolve(Window{Driver: "codex-tmux", ContextWindow: 200_000})
	if err != nil || codex.Source != SourceNoCollector || codex.FillPct != FillPctUnmeasured || !strings.Contains(codex.Warn, `driver "codex-tmux" has no usage collector (state no_collector)`) {
		t.Fatalf("codex = %+v, %v; want the state no_collector with its reason and an unmeasured fill", codex, err)
	}
	agy, err := resolve(Window{Driver: "agy-tmux", Collector: FormatAgyConversation})
	if err != nil || agy.Source != SourceNone || !strings.Contains(agy.Warn, "token usage uncovered") || strings.Contains(agy.Warn, "no usage collector") {
		t.Fatalf("agy = %+v, %v; want the uncovered warning of a format whose collector found nothing", agy, err)
	}
}

func TestResolver_TheCollectorFormatNotTheDriverNamePicksTheChain(t *testing.T) {
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
	window := Window{Driver: "newfam-tmux", Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd), ContextWindow: 1_000}

	res, err := DefaultResolver(root)(withCollector(window, FormatClaudeJSONL))
	if err != nil || res.Source != SourceTranscript {
		t.Fatalf("a new driver whose manifest names %s = %+v, %v; want the transcript read", FormatClaudeJSONL, res, err)
	}
	res, err = DefaultResolver(root)(window)
	if err != nil || res.Source != SourceNoCollector {
		t.Fatalf("the same driver with no collector = %+v, %v; want %s, because only the manifest picks a collector", res, err, SourceNoCollector)
	}
}

func withCollector(w Window, format string) Window {
	w.Collector = format
	return w
}
