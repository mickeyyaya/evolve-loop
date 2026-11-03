package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestLedgerWritingPersonasKeepTheHandoffDirectiveAfterCompaction(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cases := []struct {
		persona string
		want    bool
	}{
		{"evolve-tdd-engineer.md", true},
		{"evolve-builder.md", true},
		{"evolve-triage.md", false},
	}
	for _, c := range cases {
		raw, err := os.ReadFile(filepath.Join(root, "agents", c.persona))
		if err != nil {
			t.Fatalf("read %s: %v", c.persona, err)
		}
		dispatched := StripOnDemandSections(string(raw))
		if got := strings.Contains(dispatched, "Handoff ledger:"); got != c.want {
			t.Errorf("%s: the dispatched prompt holds the handoff directive = %v, want %v", c.persona, got, c.want)
		}
		if c.want && !strings.Contains(dispatched, "never write `author` or `output_digest`") {
			t.Errorf("%s: the dispatched directive must forbid the agent to write author and output_digest", c.persona)
		}
		if c.want && !strings.Contains(dispatched, "raise `round` on each rewrite") {
			t.Errorf("%s: the dispatched directive must tell the agent to raise round on each rewrite", c.persona)
		}
	}
}
