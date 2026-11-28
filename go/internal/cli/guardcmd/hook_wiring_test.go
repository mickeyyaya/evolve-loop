package guardcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type hookSettings struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command   string `json:"command"`
			OnFailure string `json:"onFailure"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func TestHookWiring_EveryGuardHookFailsClosed(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s hookSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse .claude/settings.json: %v", err)
	}
	guards := 0
	for _, group := range s.Hooks["PreToolUse"] {
		for _, h := range group.Hooks {
			if !strings.Contains(h.Command, " guard ") {
				continue
			}
			guards++
			if h.OnFailure != "block" {
				t.Errorf("PreToolUse guard hook on %q has onFailure %q, want \"block\": a crashed guard must deny the tool call", group.Matcher, h.OnFailure)
			}
		}
	}
	if guards != 4 {
		t.Fatalf("found %d PreToolUse guard hooks, want 4 (ship, phase, role, docdelete)", guards)
	}
}

func TestHookWiring_EveryWiredGuardIsAGuardTheBinaryBuilds(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	wired := regexp.MustCompile(`" guard ([a-z-]+)`).FindAllStringSubmatch(string(raw), -1)
	if len(wired) == 0 {
		t.Fatal("no `evolve guard <name>` hook found in .claude/settings.json; the wiring parse is broken")
	}
	for _, m := range wired {
		if _, err := buildGuard(m[1], t.TempDir(), false); err != nil {
			t.Errorf("hook runs `evolve guard %s`, which the binary cannot build: %v", m[1], err)
		}
	}
}
