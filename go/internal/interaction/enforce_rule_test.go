// enforce_rule_test.go — R8.2: the I4 shadow→enforce flip. PromoteRule
// always lands at shadow; EnforceRule is the measured-clean transition the
// batch sweep calls. Pins: flip rewrites stage preserving fields; idempotent
// on already-enforce; missing rule errors (never create on flip); corpus
// rot since promotion BLOCKS the flip.
package interaction

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const enfTestRegex = "Do you want to proceed with dangerous-op\\?"

func promoteFixture(t *testing.T) (dir, id string) {
	t.Helper()
	dir = t.TempDir()
	id, err := PromoteRule(dir, enfTestRegex, "1,Enter", "test rule", []string{"healthy line"})
	if err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	return dir, id
}

func TestEnforceRule_FlipsShadowToEnforce(t *testing.T) {
	t.Parallel()
	dir, id := promoteFixture(t)
	if err := EnforceRule(dir, id, []string{"healthy line"}); err != nil {
		t.Fatalf("EnforceRule: %v", err)
	}
	rules := LoadRules(dir, []string{"healthy line"})
	if len(rules) != 1 || rules[0].Stage != RuleStageEnforce {
		t.Fatalf("rules after flip = %+v, want one enforce-stage rule", rules)
	}
	if rules[0].Regex != enfTestRegex || rules[0].ResponseKeys != "1,Enter" {
		t.Errorf("flip must preserve regex/keys: %+v", rules[0])
	}
	// Idempotent re-flip.
	if err := EnforceRule(dir, id, []string{"healthy line"}); err != nil {
		t.Fatalf("re-EnforceRule: %v", err)
	}
}

func TestEnforceRule_MissingRuleErrors(t *testing.T) {
	t.Parallel()
	if err := EnforceRule(t.TempDir(), "rule-nonexistent", nil); err == nil {
		t.Fatal("flip of a missing rule must error — never create on flip")
	}
}

func TestEnforceRule_CorpusRotBlocksFlip(t *testing.T) {
	t.Parallel()
	dir, id := promoteFixture(t)
	// The corpus has rotted: a healthy line now matches the pattern.
	rotted := []string{"Do you want to proceed with dangerous-op? [healthy banner]"}
	err := EnforceRule(dir, id, rotted)
	if err == nil || !strings.Contains(err.Error(), "re-validation") {
		t.Fatalf("corpus rot must block the flip (got %v)", err)
	}
	// And the file must still say shadow.
	data, _ := os.ReadFile(filepath.Join(dir, id+".yaml"))
	if !strings.Contains(string(data), "stage: shadow") {
		t.Errorf("blocked flip must leave the rule at shadow:\n%s", data)
	}
}
