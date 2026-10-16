package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilesDefaultToTmux(t *testing.T) {
	t.Parallel()
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

func TestSourceWritingProfilesAllowNetwork(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(findRepoRoot(t), ".evolve", "profiles")
	for _, name := range []string{"builder", "tdd-engineer"} {
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
