package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_ProvidesCanonicalExplanationBinding(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:                           42,
		RunID:                           "01ABCDEF",
		WorktreeBaseSHA:                 strings.Repeat("a", 40),
		ExplanationDocumentationVersion: 1,
	}
	got := hooks{}.ComposePrompt("body", req)
	for _, want := range []string{
		"- worktree_base_sha: " + strings.Repeat("a", 40),
		"- explanation_documentation_version: 1",
		"- explanation_document: docs/explain/builds/cycle-42-01abcdef.md",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Build prompt missing %q:\n%s", want, got)
		}
	}
	personaPath := filepath.Join("..", "..", "..", "..", "agents", "evolve-builder.md")
	persona, err := os.ReadFile(personaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(persona), "`explanation_documentation_version: 1`") ||
		!strings.Contains(got, "- explanation_documentation_version: 1\n") {
		t.Fatalf("Build prompt activation key drifted from persona trigger")
	}
}
