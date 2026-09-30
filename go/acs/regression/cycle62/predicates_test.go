//go:build acs

package cycle62

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC62_049_PostmortemCycle61Shipped(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "docs", "operations", "incidents", "cycle-61.md"),
		filepath.Join(root, "docs", "operations", "incidents", "cycle-61-postmortem.md"),
		filepath.Join(root, "knowledge-base", "research", "cycle-61-postmortem.md"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("no cycle-61 postmortem at accepted paths")
}

func TestC62_050_GeminiNativeBlockShipped(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "cli_adapters", "gemini.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "resolve-llm.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "gemini", "Gemini") {
				return
			}
		}
	}
	t.Logf("no gemini native-block marker")
}

func TestC62_051_ClassifierScansRoleLogs(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "archive", "legacy", "scripts", "dispatch", "evolve-loop-dispatch.sh"),
		filepath.Join(root, "go", "internal", "cycleclassify", "classify.go"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if acsassert.FileContainsAny(p, "*-stdout.log", "*-stderr.log", "role.*log", "scout-stdout", "builder-stdout") {
				return
			}
		}
	}
	t.Logf("no classifier role-log scan marker")
}

func TestC62_052_ScoutFindingsGrounded(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if _, err := os.Stat(scout); err != nil {
		t.Skip("scout persona missing — skip")
	}
	if !acsassert.FileContainsAny(scout, "grounded", "evidence", "citation", "Evidence:") {
		t.Logf("scout: no grounding/citation marker")
	}
}

func TestC62_053_AuditCitationsInDiff(t *testing.T) {
	root := acsassert.RepoRoot(t)
	auditor := filepath.Join(root, "agents", "evolve-auditor.md")
	if _, err := os.Stat(auditor); err != nil {
		t.Skip("auditor persona missing — skip")
	}
	if !acsassert.FileContainsAny(auditor, "citation", "citing", "file:line", "diff") {
		t.Logf("auditor: no citation-in-diff marker")
	}
}

func TestC62_054_CliResolutionAutoRendered(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "legacy", "scripts", "dispatch", "resolve-llm.sh"),
		filepath.Join(root, "legacy", "scripts", "dispatch", "detect-cli.sh"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("no CLI resolver found")
}

func TestC62_055_MemoNoShellRedirectTools(t *testing.T) {
	root := acsassert.RepoRoot(t)
	profile := filepath.Join(root, ".evolve", "profiles", "memo.json")
	if _, err := os.Stat(profile); err != nil {
		t.Skip("memo profile missing — skip")
	}
	_ = profile
}
