//go:build integration

package integration

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestGitWorkspace_WithGitInitProducesValidWorkTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping integration test")
	}
	ws := fixtures.NewWorkspace(t).
		WithFiles(map[string]string{"README.md": "# fixture\n"}).
		WithGitInit().
		Build()

	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = ws.Root
	out, err := cmd.CombinedOutput()
	fixtures.RequireNoErr(t, err, "git rev-parse")
	if strings.TrimSpace(string(out)) != "true" {
		t.Fatalf("git rev-parse = %q, want true", strings.TrimSpace(string(out)))
	}
}
