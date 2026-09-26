package bridge

// tmux_default_env_test.go — the pane shell exports the manifest's default_env before the launch.
//
// A tmux pane inherits the tmux server's environment, so the manifest's variables reach the CLI only
// through `export` lines the driver sends after the cd and before the launch command, as
// EVOLVE_PROJECT_ROOT does. claude-tmux uses the channel to turn prompt suggestions off: a suggestion is a
// background model request per turn and dim text in the pane that reads like agent output (cycle 1707
// rendered "Yes, kill that session first." under the idle input box).

import (
	"strings"
	"testing"
)

const promptSuggestionExport = "export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false"

func TestClaudeTmuxManifest_TurnsPromptSuggestionsOff(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.DefaultEnv["CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"]; got != "false" {
		t.Fatalf("claude-tmux must declare CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false in default_env; got %q", got)
	}
}

func TestTmuxBoot_ExportsTheManifestEnvBeforeTheLaunch(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--project-root="+t.TempDir(), "--worktree="+t.TempDir())

	cdAt, rootAt, exportAt, launchAt, exports := -1, -1, -1, -1, 0
	for i, k := range tmux.sentKeys {
		switch {
		case strings.HasPrefix(k, "cd "):
			cdAt = i
		case strings.HasPrefix(k, "export EVOLVE_PROJECT_ROOT="):
			rootAt = i
		case k == promptSuggestionExport:
			exportAt = i
			exports++
		case strings.Contains(k, "claude --model"):
			launchAt = i
		}
	}
	if exports != 1 {
		t.Fatalf("the pane shell must receive %q exactly once, got %d; keys sent: %q", promptSuggestionExport, exports, tmux.sentKeys)
	}
	if !(cdAt < rootAt && rootAt < exportAt && exportAt < launchAt) {
		t.Fatalf("the export must follow the cd and the project root and precede the launch (cd=%d root=%d export=%d launch=%d)", cdAt, rootAt, exportAt, launchAt)
	}
}

func TestTmuxBoot_ACLIWithoutDefaultEnvExportsNothingExtra(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	tmux := &fakeTmux{}
	runTmuxCLI(t, fx, "codex-tmux", tmux, nil, "--allow-bypass", "--worktree="+t.TempDir())
	for _, k := range tmux.sentKeys {
		if strings.HasPrefix(k, "export ") && !strings.HasPrefix(k, "export EVOLVE_PROJECT_ROOT=") {
			t.Fatalf("codex-tmux declares no default_env, yet the pane received %q", k)
		}
	}
}
