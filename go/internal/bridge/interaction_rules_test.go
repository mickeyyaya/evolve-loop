package bridge

// interaction_rules_test.go — ADR-0045 I4 consumption side: enforce-stage
// promoted rules become live auto-respond prompts; shadow rules do not; the
// embedded healthy corpus parses and a corpus-matching rule is demoted at load.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

func TestHealthyCorpus_EmbeddedAndParsed(t *testing.T) {
	t.Parallel()
	if len(healthyCorpus) < 5 {
		t.Fatalf("embedded healthy corpus must carry the seeded lines; got %d", len(healthyCorpus))
	}
	for _, ln := range healthyCorpus {
		if ln == "" || ln[0] == '#' {
			t.Errorf("corpus must drop blank/comment lines; got %q", ln)
		}
	}
}

// TestLoadPromotedPrompts_EnforceOnly — only enforce-stage rules join the
// active set; a shadow rule rides in the registry without firing.
func TestLoadPromotedPrompts_EnforceOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := interactionRulesDir(root)

	// A sound, non-corpus enforce rule and a shadow rule.
	enforceRe := "Accept the new workspace terms to continue"
	shadowRe := "Press 9 to dismiss the upgrade banner"
	if _, err := interaction.PromoteRule(dir, enforceRe, "1,Enter", "terms", healthyCorpus); err != nil {
		t.Fatal(err)
	}
	if _, err := interaction.PromoteRule(dir, shadowRe, "9,Enter", "upgrade", healthyCorpus); err != nil {
		t.Fatal(err)
	}
	// Operator promotes the first to enforce.
	id := ""
	for _, r := range interaction.LoadRules(dir, healthyCorpus) {
		if r.Regex == enforceRe {
			id = r.ID
		}
	}
	if id == "" {
		t.Fatal("enforce rule not loaded")
	}
	bumpRuleToEnforce(t, filepath.Join(dir, id+".yaml"))

	prompts := loadPromotedPrompts(root)
	if len(prompts) != 1 {
		t.Fatalf("only the enforce rule must be active; got %d prompts: %+v", len(prompts), prompts)
	}
	if prompts[0].Regex != enforceRe || prompts[0].Policy != "auto_respond" {
		t.Errorf("promoted prompt shape wrong: %+v", prompts[0])
	}
	// Empty root → nothing, no panic.
	if got := loadPromotedPrompts(""); got != nil {
		t.Errorf("empty root must load nothing; got %+v", got)
	}
}

// bumpRuleToEnforce flips a promoted rule's stage in place (operator action).
func bumpRuleToEnforce(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "stage: shadow", "stage: enforce", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
}
