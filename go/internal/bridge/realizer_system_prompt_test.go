package bridge

import (
	"slices"
	"testing"
)

func flagValue(flags []string, flag string) (string, bool) {
	i := slices.Index(flags, flag)
	if i < 0 || i+1 >= len(flags) {
		return "", false
	}
	return flags[i+1], true
}

func TestRealizeFor_OnlyAManifestWithASystemPromptChannelCarriesTheFile(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	intent := LaunchIntent{ModelTier: "sonnet", Permission: "bypass", SystemPromptFile: "/ws/pane-authority.md"}
	r := RealizeFor("claude-tmux", intent)
	if got, ok := flagValue(r.LaunchFlags, "--append-system-prompt-file"); !ok || got != "/ws/pane-authority.md" || r.SystemPromptFile != "/ws/pane-authority.md" {
		t.Fatalf("claude-tmux realizes the system prompt file: flags=%v file=%q", r.LaunchFlags, r.SystemPromptFile)
	}
	for _, cli := range []string{"codex-tmux", "agy-tmux", "ollama-tmux"} {
		if r := RealizeFor(cli, intent); r.SystemPromptFile != "" || containsToken(r.LaunchFlags, "--append-system-prompt-file") {
			t.Errorf("%s has no system prompt channel, so it must paste the identity: flags=%v file=%q", cli, r.LaunchFlags, r.SystemPromptFile)
		}
	}
	if r := RealizeFor("claude-tmux", LaunchIntent{ModelTier: "sonnet"}); r.SystemPromptFile != "" || containsToken(r.LaunchFlags, "--append-system-prompt-file") {
		t.Errorf("no file in the intent means no flag: flags=%v", r.LaunchFlags)
	}
}
