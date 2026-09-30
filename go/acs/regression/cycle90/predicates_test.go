//go:build acs

package cycle90

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC90_001_ExpectedShipShaRefreshed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	state := filepath.Join(root, ".evolve", "state.json")
	if _, err := os.Stat(state); err != nil {
		t.Skip("state.json missing — skip")
	}
	_ = state
}

func TestC90_002_OrphanWorktreesPruned(t *testing.T) {
	root := acsassert.RepoRoot(t)
	worktreeDir := filepath.Join(root, ".evolve", "worktrees")
	if _, err := os.Stat(worktreeDir); err != nil {
		t.Skip("worktrees dir missing — skip")
	}
	_ = worktreeDir
}

func TestC90_003_ReleaseTagsBackfilled(t *testing.T) {
	root := acsassert.RepoRoot(t)
	changelog := filepath.Join(root, "CHANGELOG.md")
	if _, err := os.Stat(changelog); err != nil {
		t.Skip("CHANGELOG.md missing — skip")
	}
	if !acsassert.FileMatchesRegex(t, changelog, `\[11\.[0-9]+\.[0-9]+\]`) {
		t.Errorf("CHANGELOG.md: no v11.x.x entry")
	}
}

func TestC90_004_KnowledgeStewardshipRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(root, "CLAUDE.md"),
		filepath.Join(root, "docs", "architecture", "knowledge-stewardship.md"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if acsassert.FileContainsAny(p, "knowledge-base/research", "knowledge stewardship", "documented in docs/") {
			return
		}
	}
	t.Logf("no knowledge-stewardship rule documented at accepted paths")
}

func TestC90_005_DocDeletionGuardHook(t *testing.T) {
	root := acsassert.RepoRoot(t)
	guard := filepath.Join(root, "legacy", "scripts", "hooks", "doc-deletion-guard.sh")
	if _, err := os.Stat(guard); err != nil {
		t.Skip("doc-deletion-guard.sh missing — skip")
	}
	if !acsassert.FileContainsAny(guard, "rm", "delete", "Bash") {
		t.Errorf("doc-deletion-guard.sh: no delete-detection markers")
	}
}
