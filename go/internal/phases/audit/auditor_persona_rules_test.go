package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func auditorPersona(t *testing.T) string {
	t.Helper()
	var body strings.Builder
	for _, rel := range []string{"agents/evolve-auditor.md", "agents/evolve-auditor-reference.md"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", rel))
		if err != nil {
			t.Fatal(err)
		}
		body.Write(raw)
		body.WriteString(" ")
	}
	return strings.Join(strings.Fields(body.String()), " ")
}

func verdictAfterFinalize(t *testing.T, policyJSON string) string {
	t.Helper()
	a := &auditClassification{verdict: core.VerdictWARN, req: core.PhaseRequest{ProjectRoot: projectRootWithPolicy(t, policyJSON)}}
	a.finalize()
	return a.verdict
}

func projectRootWithPolicy(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func dispatchedAuditPrompt(t *testing.T, body, policyJSON string) string {
	t.Helper()
	got := hooks{}.ComposePrompt(body, core.PhaseRequest{
		Cycle: 1450, ProjectRoot: projectRootWithPolicy(t, policyJSON), Workspace: t.TempDir(),
	})
	return strings.Join(strings.Fields(got), " ")
}

func TestComposePrompt_StatesWhatTheAuditAndShipDoWithAWarnUnderThisProjectsPolicy(t *testing.T) {
	t.Parallel()
	if fluent, strict := verdictAfterFinalize(t, `{}`), verdictAfterFinalize(t, `{"workflow":{"strict_audit":true}}`); fluent != core.VerdictWARN || strict != core.VerdictFAIL {
		t.Fatalf("WARN stays WARN by default (%s) and becomes FAIL only under strict_audit (%s); the rendered rule states this", fluent, strict)
	}
	fluent := dispatchedAuditPrompt(t, "PERSONA", `{}`)
	strict := dispatchedAuditPrompt(t, "PERSONA", `{"workflow":{"strict_audit":true}}`)
	if !strings.Contains(fluent, "WARN ships under the fluent default; only `workflow.strict_audit` in `.evolve/policy.json` makes ship refuse it") {
		t.Errorf("the fluent prompt does not state that a WARN ships:\n%s", fluent)
	}
	if !strings.Contains(strict, "this project sets `workflow.strict_audit` in `.evolve/policy.json`, so the host turns your WARN into FAIL and ship refuses it") {
		t.Errorf("the strict prompt does not state that a WARN becomes FAIL:\n%s", strict)
	}
	if strings.Contains(strict, "WARN ships under the fluent default") {
		t.Error("the strict prompt tells the auditor a WARN ships")
	}
}

func TestComposePrompt_MakesAnAddedCommentALowFindingAndNeverAsksForOne(t *testing.T) {
	t.Parallel()
	prompt := dispatchedAuditPrompt(t, "PERSONA", `{}`)
	for _, want := range []string{
		"New and changed code carries no comments ([docs/conventions/code-comments.md]",
		"§What a comment may say lists the only exceptions",
		"each added comment is a LOW finding, advisory and never a WARN or FAIL on its own",
		"never ask for a doc comment or a comment that explains code",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the dispatched prompt does not say %q", want)
		}
	}
}

func TestComposePrompt_KeepsAnInaccurateExplanationItsOwnFailWhileTheGateRecordsAnAdvisory(t *testing.T) {
	t.Parallel()
	report, req := needsCorrectionReview()
	if advisories, err := validateExplanationReview(report, req); err != nil || !containsAdvisory(advisories, "NEEDS_CORRECTION") {
		t.Fatalf("the gate records NEEDS_CORRECTION as an advisory and never blocks (ADR-0102): advisories=%v err=%v", advisories, err)
	}
	prompt := dispatchedAuditPrompt(t, "PERSONA", `{}`)
	for _, want := range []string{
		"records `NEEDS_CORRECTION` as an advisory (ADR-0102), so the verdict is yours: make it FAIL",
		"name only the document",
		"begin each such defect with the document's path and line",
		"the host routes the repair round to Build's explanation re-author alone (`explanation-needs-correction`)",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the dispatched prompt does not say %q", want)
		}
	}
}

func TestComposePrompt_TheRulesLiveOnlyInTheRenderedBlockAndNoStalePhrasingReappears(t *testing.T) {
	t.Parallel()
	persona := auditorPersona(t)
	for _, restated := range []string{
		"WARN ships under the fluent default",
		"each added comment is a LOW finding",
		"records `NEEDS_CORRECTION` as an advisory",
	} {
		if strings.Contains(persona, restated) {
			t.Errorf("the persona restates the rule %q; the host renders it from one source", restated)
		}
	}
	for _, policyJSON := range []string{`{}`, `{"workflow":{"strict_audit":true}}`} {
		prompt := dispatchedAuditPrompt(t, persona, policyJSON)
		if !strings.Contains(prompt, "## Audit Rules") {
			t.Fatalf("the dispatched prompt has no rendered rules block (policy %s)", policyJSON)
		}
		for _, stale := range []string{
			"WARN blocks shipping",
			"WARN blocks ship",
			"honest negative judgment forces the overall Audit to fail",
			"NEEDS_CORRECTION forces",
			"add a doc comment",
		} {
			if strings.Contains(prompt, stale) {
				t.Errorf("the dispatched prompt carries the stale phrasing %q (policy %s)", stale, policyJSON)
			}
		}
	}
}

func TestAuditAndShipSkills_PointAtTheRenderedWarnRuleInsteadOfRestatingIt(t *testing.T) {
	t.Parallel()
	for rel, rowPrefix := range map[string]string{"skills/audit/SKILL.md": "| `WARN` |", "skills/ship/SKILL.md": "| 1 |"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", rel))
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		if strings.Contains(body, "strict_audit") {
			t.Errorf("%s restates the WARN ship rule; it must point at go/internal/phases/audit/audit_rules.go", rel)
		}
		var row string
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, rowPrefix) {
				row = line
			}
		}
		if !strings.Contains(row, "go/internal/phases/audit/audit_rules.go") {
			t.Errorf("%s: the row %q does not point at the one source of the WARN rule: %q", rel, rowPrefix, row)
		}
	}
}
