package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBridgeCommandIntent_CarriesTheResolvedEffort(t *testing.T) {
	cases := []struct {
		cli, permission, want string
		flags                 []string
	}{
		{"claude-tmux", "bypass", "medium", []string{"--effort", "medium"}},
		{"agy-claude-tmux", "", "low", nil},
		{"codex-tmux", "bypass", "medium", []string{"-c", "model_reasoning_effort=medium"}},
	}
	for _, tc := range cases {
		intent := bridgeCommandIntent(tc.cli, tc.permission, policy.EffortTable{})
		if intent.Effort != tc.want || intent.Permission != tc.permission {
			t.Errorf("%s intent = %+v, want effort %q and permission %q", tc.cli, intent, tc.want, tc.permission)
		}
		flags := bridge.RealizeFor(tc.cli, intent).LaunchFlags
		if tc.flags != nil && !slices.ContainsFunc(flags, func(f string) bool { return f == tc.flags[1] }) {
			t.Errorf("%s flags = %v, want %v", tc.cli, flags, tc.flags)
		}
	}
}

func TestCommandEfforts_ReadsTheProjectPolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"cli_routing":{"clis":["claude"],"tiers":{"balanced":{"effort":"high"}}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	var stderr bytes.Buffer
	if got := bridgeCommandIntent("claude-tmux", "", commandEfforts(&stderr)).Effort; got != "high" {
		t.Fatalf("a bridge command effort = %q, want high from cli_routing.tiers.balanced; stderr=%s", got, stderr.String())
	}
}
