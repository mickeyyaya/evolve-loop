package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTDDPromptDeclaresHandoffSlugs — cycle-1620 salvage (architecture
// CRITICAL 2): the TDD->Build scope gate reads `slugs[]` from the handoff JSON
// and a comma-separated `## Task:` header, so the persona that PRODUCES the
// report must instruct exactly that shape, bound to the `## Task Contract`
// block's ids. Without this the only reports that pass the multi-member gate
// are the ones tests author themselves.
func TestTDDPromptDeclaresHandoffSlugs(t *testing.T) {
	path := filepath.Join(repoRoot(t), "agents", "evolve-tdd-engineer.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(data)
	for _, want := range []string{`"slugs"`, "## Task: <id>[, <id>", "Task Contract"} {
		if !strings.Contains(body, want) {
			t.Errorf("evolve-tdd-engineer.md must carry %q — the declaration the scope gate reconciles", want)
		}
	}
}
