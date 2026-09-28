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
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &auditClassification{verdict: core.VerdictWARN, req: core.PhaseRequest{ProjectRoot: root}}
	a.finalize()
	return a.verdict
}

func TestAuditorPersona_StatesWhatTheAuditAndShipDoWithAWarn(t *testing.T) {
	if fluent, strict := verdictAfterFinalize(t, `{}`), verdictAfterFinalize(t, `{"workflow":{"strict_audit":true}}`); fluent != core.VerdictWARN || strict != core.VerdictFAIL {
		t.Fatalf("WARN stays WARN by default (%s) and becomes FAIL only under strict_audit (%s); the persona sentence below states this", fluent, strict)
	}
	if !strings.Contains(auditorPersona(t), "WARN ships under the fluent default; only `workflow.strict_audit` in `.evolve/policy.json` makes ship refuse it") {
		t.Error("the persona does not state what ship does with a WARN")
	}
}

func TestAuditorPersona_MakesAnAddedCommentALowFindingAndNeverAsksForOne(t *testing.T) {
	persona := auditorPersona(t)
	for _, want := range []string{
		"New and changed code carries no comments ([docs/conventions/code-comments.md]",
		"§What a comment may say lists the only exceptions",
		"each added comment is a LOW finding, advisory and never a WARN or FAIL on its own",
		"never ask for a doc comment or a comment that explains code",
	} {
		if !strings.Contains(persona, want) {
			t.Errorf("the persona does not say %q", want)
		}
	}
}

func TestAuditorPersona_KeepsAnInaccurateExplanationItsOwnFailWhileTheGateRecordsAnAdvisory(t *testing.T) {
	report, req := needsCorrectionReview()
	if advisories, err := validateExplanationReview(report, req); err != nil || !containsAdvisory(advisories, "NEEDS_CORRECTION") {
		t.Fatalf("the gate records NEEDS_CORRECTION as an advisory and never blocks (ADR-0102): advisories=%v err=%v", advisories, err)
	}
	persona := auditorPersona(t)
	for _, want := range []string{
		"records `NEEDS_CORRECTION` as an advisory (ADR-0102), so the verdict is yours: make it FAIL",
		"name only the document",
	} {
		if !strings.Contains(persona, want) {
			t.Errorf("the persona does not say %q", want)
		}
	}
}
