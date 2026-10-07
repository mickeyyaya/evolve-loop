package deliverable

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
)

func reviewSpec(grammars ...string) phasespec.PhaseSpec {
	return phasespec.PhaseSpec{
		Name: "code-review", Role: "evaluate", Optional: true,
		Outputs:  phasespec.IO{Files: []string{".evolve/runs/cycle-{cycle}/code-review-report.md"}},
		Classify: &phasespec.ClassifyRules{RequireSections: []string{"Review Plan", "Findings", "Scores", "Verdict"}, Grammars: grammars},
	}
}

func reviewReport(securityScore string) string {
	var b strings.Builder
	b.WriteString("## Review Plan\n- Inputs read: build-report.md, the diff\n")
	for _, k := range qualityindex.Keys() {
		b.WriteString("- " + k + ": required (light) — the diff touches it\n")
	}
	b.WriteString("\n## Findings\nNone.\n\n## Scores\n")
	for _, k := range qualityindex.Keys() {
		score := "4 — the bar holds"
		if k == "security" {
			score = securityScore
		}
		b.WriteString("- " + k + ": " + score + "\n")
	}
	b.WriteString("\n## Verdict\nPASS\n\n<!-- evolve-verdict: {\"phase\":\"code-review\",\"verdict\":\"PASS\",\"schema_version\":1} -->\n")
	return b.String()
}

func verifyReview(t *testing.T, spec phasespec.PhaseSpec, report string, evolveDir string) Result {
	t.Helper()
	ws := t.TempDir()
	writeFile(t, ws, "code-review-report.md", report)
	res, err := VerifyWith("code-review", phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir}, resolverFor(spec))
	if err != nil {
		t.Fatalf("VerifyWith: %v", err)
	}
	return res
}

func TestVerify_ADeclaredGrammarPassesAWellFormedReport(t *testing.T) {
	if res := verifyReview(t, reviewSpec(phasespec.GrammarCodeReviewReport), reviewReport("4 — fine"), t.TempDir()); !res.OK {
		t.Errorf("violations = %+v, want a well-formed review report accepted", res.Violations)
	}
}

func TestVerify_ADeclaredGrammarSendsAMalformedReportToCorrection(t *testing.T) {
	res := verifyReview(t, reviewSpec(phasespec.GrammarCodeReviewReport), reviewReport("3 — weak, no finding cited"), t.TempDir())
	if res.OK || !hasCode(res, CodeBadGrammar) {
		t.Fatalf("violations = %+v, want a %s violation: a gap with no finding is malformed", res.Violations, CodeBadGrammar)
	}
	if !strings.Contains(res.Violations[0].Message, "security") || !strings.Contains(res.Violations[0].Message, phasespec.GrammarCodeReviewReport) {
		t.Errorf("message %q, want it to name the dimension and the grammar: it is the correction directive", res.Violations[0].Message)
	}
}

func TestVerify_TheGrammarReadsTheProjectsConfiguredThresholds(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"workflow":{"quality_index":{"thresholds":{"security":3}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := verifyReview(t, reviewSpec(phasespec.GrammarCodeReviewReport), reviewReport("3 — at the configured bar"), evolveDir); !res.OK {
		t.Errorf("violations = %+v, want a 3 accepted against the project's threshold of 3", res.Violations)
	}
}

func TestVerify_AMalformedPolicyIsTheGatesOwnFaultNeverTheAgents(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	writeFile(t, ws, "code-review-report.md", reviewReport("4 — fine"))

	res, err := VerifyWith("code-review", phasecontract.Roots{Workspace: ws, EvolveDir: evolveDir}, resolverFor(reviewSpec(phasespec.GrammarCodeReviewReport)))

	if err == nil || !strings.Contains(err.Error(), "policy.json") || len(res.Violations) != 0 {
		t.Errorf("VerifyWith = (%+v, %v), want the policy load error and no violation: an agent cannot repair the project's policy, so the gate fails open loudly", res, err)
	}
}

func TestVerify_AnUndeclaredGrammarChecksNothingAndAnUnboundOneIsARegistryDefect(t *testing.T) {
	if res := verifyReview(t, reviewSpec(), reviewReport("1 — no finding cited"), t.TempDir()); !res.OK {
		t.Errorf("violations = %+v, want no grammar check on a phase that declares none", res.Violations)
	}
	res := verifyReview(t, reviewSpec("no-such-grammar"), reviewReport("4 — fine"), t.TempDir())
	if res.OK || !hasCode(res, CodeUnboundGrammar) {
		t.Errorf("violations = %+v, want %s for a grammar with no registered check", res.Violations, CodeUnboundGrammar)
	}
}

func TestGrammars_TheGateBindsExactlyTheGrammarsASpecMayDeclare(t *testing.T) {
	bound := make([]string, 0, len(grammars))
	for name := range grammars {
		bound = append(bound, name)
	}
	sort.Strings(bound)
	declarable := phasespec.Grammars()
	sort.Strings(declarable)

	if !reflect.DeepEqual(bound, declarable) {
		t.Errorf("the gate binds %v, a spec may declare %v: a declarable grammar with no check, or a check no spec may name, is a registry defect", bound, declarable)
	}
}
