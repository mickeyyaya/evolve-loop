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

// TestSourceWritingProfilesAllowNetwork guards the allow_network invariant for
// the built-in source-writing phases. Those phases run under the OS sandbox
// (worktree assigned), and the sandbox confines the LLM CLI itself — so
// sandbox.allow_network MUST be true or the sandboxed CLI boots network-denied
// and can never reach the model API. Network-denial is not an available control
// for a model-reaching phase (per-host egress isn't expressible in sandbox-exec/
// bwrap, and the model endpoint can't be proxied without breaking subscription
// billing); the boundary for these phases is filesystem confinement + kernel
// hooks. This pins the value so a future flip back to false (the latent bug that
// shipped silently for lack of a JSON tag) fails the build.
func TestSourceWritingProfilesAllowNetwork(t *testing.T) {
	dir := filepath.Join(findRepoRoot(t), ".evolve", "profiles")
	for _, name := range []string{"builder", "tdd-engineer"} { // built-in worktree phases (build, tdd)
		b, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if err != nil {
			t.Fatalf("read %s.json: %v", name, err)
		}
		var prof struct {
			Sandbox *struct {
				AllowNetwork *bool `json:"allow_network"`
			} `json:"sandbox"`
		}
		if parseErr := json.Unmarshal(b, &prof); parseErr != nil {
			t.Fatalf("parse %s.json: %v", name, parseErr)
		}
		if prof.Sandbox == nil || prof.Sandbox.AllowNetwork == nil || !*prof.Sandbox.AllowNetwork {
			t.Errorf("%s.json is a source-writing phase but sandbox.allow_network is not true — "+
				"its sandboxed CLI would boot network-denied and fail to reach the model", name)
		}
	}
}
