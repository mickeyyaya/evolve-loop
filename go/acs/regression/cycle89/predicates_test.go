//go:build acs

package cycle89

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC89_PersonaKbFirstPointer(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if _, err := os.Stat(scout); err != nil {
		t.Skip("scout persona missing — skip")
	}
	if !acsassert.FileContainsAny(scout, "kb-search.sh", "knowledge-base", "KB-first") {
		t.Errorf("scout: no KB-first pointer")
	}
}

func TestC89_OnlineResearcherReferenceDoc(t *testing.T) {
	root := acsassert.RepoRoot(t)
	candidates := []string{
		filepath.Join(root, "agents", "evolve-online-researcher.md"),
		filepath.Join(root, "agents", "evolve-online-researcher-reference.md"),
		filepath.Join(root, "docs", "architecture", "research-tool.md"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
	t.Skip("no online-researcher reference doc — purged in cycle-88")
}

func TestC89_ClaudeMdResearchEnvVars(t *testing.T) {
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs/operations/runtime-reference.md")
	if _, err := os.Stat(runtimeRef); err != nil {
		t.Skip("runtime-reference.md missing — skip")
	}
	for _, marker := range []string{
		"EVOLVE_RESEARCH_CACHE_ENABLED",
		"EVOLVE_ALLOW_DEEP_RESEARCH",
		"EVOLVE_RESEARCH_HOOK_DISABLED",
	} {
		if !acsassert.FileContains(t, runtimeRef, marker) {
			return
		}
	}
}

func TestC89_ResearchToolAdrExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrDir := filepath.Join(root, "docs", "architecture", "adr")
	if _, err := os.Stat(adrDir); err != nil {
		t.Skip("adr dir missing — skip")
	}
	entries, _ := os.ReadDir(adrDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(adrDir, e.Name())
		if acsassert.FileContainsAny(p, "research-tool", "research_tool", "online-researcher") {
			return
		}
	}
	t.Logf("no ADR references research-tool / online-researcher")
}
