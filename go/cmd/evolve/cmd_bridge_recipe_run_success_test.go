//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestRunBridgeRecipeRun_ACompletedRecipeExitsZeroWithItsResult(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\ncase \"$*\" in *capture-pane*) printf '\\342\\235\\257 \\n' ;; esac\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	ws := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := runBridgeRecipeRun([]string{"list-capabilities", "--cli=claude-tmux", "--workspace=" + ws, "--worktree=" + ws, "--session-name=evolve-recipe-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%s", code, stderr.String())
	}
	var res struct {
		Recipe string `json:"recipe"`
		CLI    string `json:"cli"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatalf("stdout is not a recipe result: %v\n%s", err, stdout.String())
	}
	if res.Recipe != "list-capabilities" || res.CLI != "claude-tmux" || res.Status != "complete" {
		t.Errorf("result = %+v, want a complete list-capabilities run on claude-tmux", res)
	}
}
