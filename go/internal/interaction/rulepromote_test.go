package interaction_test

// ADR-0045 I4 (§8): interaction-rule promotion — the REJECTING validation gate
// + absent-only durable registry + boot re-validation against the corpus.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

// healthyCorpus is a small stand-in for the immutable operator-reviewed
// fixture: lines that appear in NORMAL panes of the driver CLIs.
var healthyCorpus = []string{
	"● Writing scout-report.md to the workspace…",
	"❯ ",
	"esc to interrupt",
	"Deliberating 12s",
}

// TestRuleValidate_RejectsShortPatternHealthyCorpusMatch — the two
// false-positive-bomb refusals: a too-short pattern, and a pattern that
// matches the healthy corpus.
func TestRuleValidate_RejectsShortPatternHealthyCorpusMatch(t *testing.T) {
	t.Parallel()
	if err := interaction.ValidateRule("y.*", "y,Enter", healthyCorpus); err == nil {
		t.Error("a too-short pattern must be refused")
	}
	if err := interaction.ValidateRule("Writing scout-report", "Enter", healthyCorpus); err == nil {
		t.Error("a pattern matching the healthy corpus must be refused (would fire on normal output)")
	}
	// A sound, specific, non-corpus pattern with valid keys passes.
	if err := interaction.ValidateRule("Rate this session before exiting", "1,Enter", healthyCorpus); err != nil {
		t.Errorf("a sound rule must validate: %v", err)
	}
}

// TestRuleValidate_KeyspecSuspectREJECTED — the hard gate (not keyspec's
// advisory WARN): any ClassSuspect response token refuses the whole rule.
func TestRuleValidate_KeyspecSuspectREJECTED(t *testing.T) {
	t.Parallel()
	// "Excape" is the canonical keyspec ClassSuspect example (mistyped Escape).
	if err := interaction.ValidateRule("Press Escape to dismiss the dialog", "Excape", healthyCorpus); err == nil {
		t.Error("a ClassSuspect response key must REJECT the rule (stricter than keyspec.Validate's WARN-but-send)")
	}
	// The correctly-spelled named key passes the gate.
	if err := interaction.ValidateRule("Press Escape to dismiss the dialog", "Escape", healthyCorpus); err != nil {
		t.Errorf("a valid named key must pass: %v", err)
	}
	// An invalid regex is refused.
	if err := interaction.ValidateRule("unterminated(", "Enter", healthyCorpus); err == nil {
		t.Error("a non-compiling regex must be refused")
	}
}

// TestPromotedRule_LandsShadow — promotion writes an absent-only file at
// stage shadow; re-promotion is idempotent; an operator-edited file wins.
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

	// Idempotent re-promotion: same id, no duplicate.
	id2, err := interaction.PromoteRule(dir, regex, "1,Enter", "again", healthyCorpus)
	if err != nil || id2 != id {
		t.Errorf("re-promotion must be idempotent (id %q vs %q, err %v)", id2, id, err)
	}
	// Operator edits the file to enforce — the edit wins (absent-only write).
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

// TestBootReplay_RevalidatesAgainstCorpus_DemotesNowMatching — S3 corpus-rot:
// a rule promoted clean against an OLD corpus must be dropped from the active
// set when a corpus update makes its pattern match a now-healthy banner.
func TestBootReplay_RevalidatesAgainstCorpus_DemotesNowMatching(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	regex := "Press 2 to accept the new terms"
	if _, err := interaction.PromoteRule(dir, regex, "2,Enter", "terms prompt", healthyCorpus); err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	// Same (old) corpus → rule loads active.
	if got := interaction.LoadRules(dir, healthyCorpus); len(got) != 1 {
		t.Fatalf("rule must load under the corpus it was promoted against; got %d", len(got))
	}
	// A new CLI version starts printing the exact banner the rule matches.
	rottedCorpus := append([]string{"Press 2 to accept the new terms and continue"}, healthyCorpus...)
	if got := interaction.LoadRules(dir, rottedCorpus); len(got) != 0 {
		t.Errorf("a rule now matching the corpus must be demoted out of the active set; got %+v", got)
	}
}

// TestRuleFiles_AbsentOnlyAndCorruptSafe — corrupt files are skipped (boot
// never bricks); a non-yaml file is ignored.
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
	// Absent dir → nil, no panic.
	if got := interaction.LoadRules(filepath.Join(dir, "nope"), healthyCorpus); got != nil {
		t.Errorf("absent dir must load nothing; got %+v", got)
	}
}
