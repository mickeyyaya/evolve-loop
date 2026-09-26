package bridge

import (
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
)

// The identity block is part of the injected prompt, and the auto-responder drops pane lines that echo the
// injected prompt before it scans for a wall. No manifest pattern may therefore match the block's own text,
// or an echoed block line could hide the wall it sits beside.
func TestIdentityBlockMatchesNoManifestPattern(t *testing.T) {
	block := phaseidentity.Block(phaseidentity.Facts{
		Agent: "build", Cycle: 1, Session: "evolve-bridge-x", PromptFile: "/w/build-prompt.txt",
		PastedFile: "/w/resolved-prompt.txt", Artifact: "/w/build-report.md",
	})
	for _, cli := range []string{"claude-tmux", "codex-tmux", "agy-tmux", "ollama-tmux"} {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatal(err)
		}
		patterns := map[string]string{"transient_regex": m.TransientRegex}
		for _, p := range m.InteractivePrompts {
			patterns["interactive_prompts."+p.Name] = p.Regex
		}
		if usage, ok := m.Controls["usage"]; ok {
			patterns["controls.usage.exhausted_regex"] = usage.ExhaustedRegex
		}
		for name, pattern := range patterns {
			if pattern == "" {
				continue
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(block) {
				t.Fatalf("%s %s matches the identity block: an echoed block line could mask that wall", cli, name)
			}
		}
	}
}
