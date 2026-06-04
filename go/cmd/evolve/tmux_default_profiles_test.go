// tmux_default_profiles_test.go — guard test enforcing the tmux-LLM-default
// policy. Asserts that every .evolve/profiles/*.json pins a "-tmux" cli OR
// leaves cli empty (llmroute falls back to "claude-tmux" — still tmux).
// Fails the build when a profile pins a headless driver as its default,
// making such a change a conscious, reviewed act.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProfilesDefaultToTmux asserts the tmux-LLM-default policy for every
// phase profile. Headless drivers (claude-p, codex, agy) are opt-in only —
// selectable via EVOLVE_CLI / EVOLVE_<AGENT>_CLI / a .evolve/policy.json pin
// — and must never be a profile default.
//
// findRepoRoot is shared with docs_contract_test.go (same package).
func TestProfilesDefaultToTmux(t *testing.T) {
	dir := filepath.Join(findRepoRoot(t), ".evolve", "profiles")
	profiles, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("glob profiles: %v", err)
	}
	if len(profiles) == 0 {
		t.Fatalf("no profiles found under %s", dir)
	}
	for _, p := range profiles {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var prof struct {
			CLI string `json:"cli"`
		}
		if parseErr := json.Unmarshal(b, &prof); parseErr != nil {
			t.Fatalf("parse %s: %v", p, parseErr)
		}
		if prof.CLI != "" && !strings.HasSuffix(prof.CLI, "-tmux") {
			t.Errorf("%s pins a non-tmux default cli %q — the tmux-LLM path is the default; "+
				"headless is opt-in via EVOLVE_CLI / EVOLVE_<AGENT>_CLI / a policy pin, not a profile default",
				filepath.Base(p), prof.CLI)
		}
	}
}
