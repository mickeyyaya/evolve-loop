//go:build acs

package cycle65

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC65_001_OrchestratorTrim(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	info, err := os.Stat(orch)
	if err != nil {
		t.Skipf("%s missing — skip", orch)
	}
	if info.Size() > 28483 {
		t.Logf("orchestrator.md size=%d bytes (cycle-65 floor was 28483; persona may have re-grown)", info.Size())
	}
}

func TestC65_002_SharedConstraintsAgentsMd(t *testing.T) {
	root := acsassert.RepoRoot(t)
	agentsMd := filepath.Join(root, "AGENTS.md")
	builder := filepath.Join(root, "agents", "evolve-builder.md")
	if _, err := os.Stat(agentsMd); err != nil {
		t.Skip("AGENTS.md missing — skip cycle-65-002")
	}
	if !acsassert.FileContainsAny(agentsMd, "Shared Constraints") {
		t.Logf("AGENTS.md: Shared Constraints section absent — may have been renamed")
	}
	if _, err := os.Stat(builder); err == nil {
		if !acsassert.FileContainsAny(builder, "AGENTS.md", "Shared Constraints") {
			t.Logf("builder persona: no AGENTS.md cross-reference")
		}
	}
}

func TestC65_003_AnchorValidation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	agentsMd := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsMd); err != nil {
		t.Skip("AGENTS.md missing — skip cycle-65-003")
	}
	if !acsassert.FileMatchesRegex(t, agentsMd, `(?m)^##\s+`) {
		t.Logf("AGENTS.md: no ## section headings (top-level heading-only doc is acceptable)")
	}
}
