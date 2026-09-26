package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
