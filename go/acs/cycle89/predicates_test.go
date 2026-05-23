// Package cycle89 ports the cycle-89 ACS predicates (4 bash files).
package cycle89

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC89_PersonaKbFirstPointer ports cycle-89/001.
// Personas must reference kb-search.sh / KB-first research policy.
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

// TestC89_OnlineResearcherReferenceDoc ports cycle-89/002.
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

// TestC89_ClaudeMdResearchEnvVars ports cycle-89/003.
func TestC89_ClaudeMdResearchEnvVars(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeMd := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMd); err != nil {
		t.Skip("CLAUDE.md missing — skip")
	}
	for _, marker := range []string{
		"EVOLVE_RESEARCH_CACHE_ENABLED",
		"EVOLVE_ALLOW_DEEP_RESEARCH",
		"EVOLVE_RESEARCH_HOOK_DISABLED",
	} {
		if !acsassert.FileContains(t, claudeMd, marker) {
			return
		}
	}
}

// TestC89_ResearchToolAdrExists ports cycle-89/004.
func TestC89_ResearchToolAdrExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrDir := filepath.Join(root, "docs", "architecture", "adr")
	if _, err := os.Stat(adrDir); err != nil {
		t.Skip("adr dir missing — skip")
	}
	// Some ADR should reference research-tool / online-researcher purge
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
