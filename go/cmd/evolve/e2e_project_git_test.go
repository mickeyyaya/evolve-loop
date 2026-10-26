//go:build e2e

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestE2EProject_GitHasNoMaintenance(t *testing.T) {
	t.Parallel()
	root := setupTempProject(t, "")

	for _, kv := range gittest.MaintenanceConfig() {
		out, err := exec.Command("git", "-C", root, "config", "--local", "--get", kv[0]).Output()
		if got := strings.TrimSpace(string(out)); err != nil || got != kv[1] {
			t.Errorf("e2e project git config %s = %q (err %v), want %q so no maintenance child races the temp dir cleanup", kv[0], got, err, kv[1])
		}
	}
}
