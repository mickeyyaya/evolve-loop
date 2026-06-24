//go:build integration

package integration

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// Integration tier example (//go:build integration → excluded from the fast
// default suite; run via `make test-integration`). This exercises the
// harness's real-git support end to end: WithGitInit must produce a valid
// work tree that the git binary itself recognizes.
//
// Pattern to copy when adding an integration test:
//  1. gate on the external tool: if it's absent, t.Skip (don't fail).
//  2. ws := fixtures.NewWorkspace(t).WithGitInit().Build()
//  3. shell out to the real tool against ws.Root and assert.
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
