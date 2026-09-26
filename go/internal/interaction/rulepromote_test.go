package interaction_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// healthyCorpus stands in for the embedded corpus of lines from normal driver-CLI panes.
var healthyCorpus = []string{
	"● Writing scout-report.md to the workspace…",
	"❯ ",
	"esc to interrupt",
	"Deliberating 12s",
}

func TestRuleValidate_RejectsShortPatternHealthyCorpusMatch(t *testing.T) {
	t.Parallel()
	if err := interaction.ValidateRule("y.*", "y,Enter", healthyCorpus); err == nil {
		t.Error("a too-short pattern must be refused")
	}
	if err := interaction.ValidateRule("Writing scout-report", "Enter", healthyCorpus); err == nil {
		t.Error("a pattern matching the healthy corpus must be refused (would fire on normal output)")
	}
	if err := interaction.ValidateRule("Rate this session before exiting", "1,Enter", healthyCorpus); err != nil {
		t.Errorf("a sound rule must validate: %v", err)
	}
}

func TestRuleValidate_KeyspecSuspectREJECTED(t *testing.T) {
	t.Parallel()
	// "Excape" is the canonical keyspec ClassSuspect example (mistyped Escape).
	if err := interaction.ValidateRule("Press Escape to dismiss the dialog", "Excape", healthyCorpus); err == nil {
		t.Error("a ClassSuspect response key must REJECT the rule (stricter than keyspec.Validate's WARN-but-send)")
	}
	if err := interaction.ValidateRule("Press Escape to dismiss the dialog", "Escape", healthyCorpus); err != nil {
		t.Errorf("a valid named key must pass: %v", err)
	}
	if err := interaction.ValidateRule("unterminated(", "Enter", healthyCorpus); err == nil {
		t.Error("a non-compiling regex must be refused")
	}
}

func TestPromotedRule_LandsShadow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regex := "Rate this session before exiting"
	id, err := interaction.PromoteRule(dir, regex, "1,Enter", "agy session rating", healthyCorpus)
	if err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	rules := interaction.LoadRules(dir, healthyCorpus)
	if len(rules) != 1 {
		t.Fatalf("loaded %d rules, want 1", len(rules))
	}
	if rules[0].Stage != interaction.RuleStageShadow {
		t.Errorf("a freshly promoted rule must land shadow; got %q", rules[0].Stage)
	}
	if rules[0].ID != id || rules[0].Regex != regex {
		t.Errorf("loaded rule mismatch: %+v", rules[0])
	}

	id2, err := interaction.PromoteRule(dir, regex, "1,Enter", "again", healthyCorpus)
	if err != nil || id2 != id {
		t.Errorf("re-promotion must be idempotent (id %q vs %q, err %v)", id2, id, err)
	}
	path := filepath.Join(dir, id+".yaml")
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), "stage: shadow", "stage: enforce", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := interaction.PromoteRule(dir, regex, "1,Enter", "x", healthyCorpus); err != nil {
		t.Fatalf("re-promote over operator file: %v", err)
	}
	rules = interaction.LoadRules(dir, healthyCorpus)
	if len(rules) != 1 || rules[0].Stage != interaction.RuleStageEnforce {
		t.Errorf("operator enforce edit must survive re-promotion; got %+v", rules)
	}
}

func TestBootReplay_RevalidatesAgainstCorpus_DemotesNowMatching(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regex := "Press 2 to accept the new terms"
	if _, err := interaction.PromoteRule(dir, regex, "2,Enter", "terms prompt", healthyCorpus); err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	if got := interaction.LoadRules(dir, healthyCorpus); len(got) != 1 {
		t.Fatalf("rule must load under the corpus it was promoted against; got %d", len(got))
	}
	rottedCorpus := append([]string{"Press 2 to accept the new terms and continue"}, healthyCorpus...)
	if got := interaction.LoadRules(dir, rottedCorpus); len(got) != 0 {
		t.Errorf("a rule now matching the corpus must be demoted out of the active set; got %+v", got)
	}
}

func TestRuleFiles_AbsentOnlyAndCorruptSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := interaction.PromoteRule(dir, "Rate this session before exiting", "1,Enter", "n", healthyCorpus); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.yaml"), []byte("}{not yaml::"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := interaction.LoadRules(dir, healthyCorpus)
	if len(rules) != 1 {
		t.Errorf("corrupt + non-yaml files must be skipped, valid rule kept; got %d", len(rules))
	}
	if got := interaction.LoadRules(filepath.Join(dir, "nope"), healthyCorpus); got != nil {
		t.Errorf("absent dir must load nothing; got %+v", got)
	}
}
