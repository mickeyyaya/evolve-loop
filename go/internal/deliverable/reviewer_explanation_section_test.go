package deliverable

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const auditWithoutExplanation = "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n"
const auditWithExplanation = auditWithoutExplanation + "\n## Explanation Documentation\n- Status: VERIFIED\n- Evidence: docs/explain/builds/cycle-42.md:1 matches go/app.go:19\n"

func writeAuditReport(t *testing.T, body string) (workspace, projectRoot string) {
	t.Helper()
	workspace, projectRoot = t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "audit-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return workspace, projectRoot
}

// TestReviewer_ExplanationSectionRequiredOnlyWhileContractActive — cycles 1601
// and 1603 died on "audit-report.md is missing ## Explanation Documentation"
// as a terminal FAIL. The section is a contract violation the correction
// ladder re-dispatches — but only when the cycle's explanation contract is
// active; a cycle without it is never asked for the section.
func TestReviewer_ExplanationSectionRequiredOnlyWhileContractActive(t *testing.T) {
	t.Parallel()
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)

	ws, root := writeAuditReport(t, auditWithoutExplanation)
	active := core.ReviewInput{Phase: "audit", Workspace: ws, ProjectRoot: root, ExplanationDocumentationVersion: 1}
	res := r.Review(context.Background(), active)
	if res.Approve {
		t.Fatal("an audit report without the section, while the contract is active, must be REJECTED so the ladder re-dispatches the auditor")
	}
	if !strings.Contains(res.Reason, `[missing_section] required section "## Explanation Documentation" is missing`) || !strings.Contains(res.Reason, "contract v1 is active") {
		t.Fatalf("the rejection is the correction directive and must name the section and why it is owed: %q", res.Reason)
	}

	inactive := active
	inactive.ExplanationDocumentationVersion = 0
	if res := r.Review(context.Background(), inactive); !res.Approve {
		t.Fatalf("a cycle without the explanation contract must not be asked for the section: %q", res.Reason)
	}

	ws2, root2 := writeAuditReport(t, auditWithExplanation)
	if res := r.Review(context.Background(), core.ReviewInput{Phase: "audit", Workspace: ws2, ProjectRoot: root2, ExplanationDocumentationVersion: 1}); !res.Approve {
		t.Fatalf("a report carrying the section must pass: %q", res.Reason)
	}
}

// TestVerify_ExplanationSectionRidesOnRoots — the version travels on Roots, so
// Verify (the CLI self-check, the salvage re-check) judges the section exactly
// as the host gate does; the match is the exact visible level-two heading.
func TestVerify_ExplanationSectionRidesOnRoots(t *testing.T) {
	t.Parallel()
	verify := func(body string, version int) Result {
		ws, root := writeAuditReport(t, body)
		res, err := VerifyWithStage("audit", phasecontract.Roots{Workspace: ws, EvolveDir: filepath.Join(root, ".evolve"), ExplanationDocumentationVersion: version}, phasecontract.BuiltinResolver{}, config.StageOff)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := verify(auditWithoutExplanation, 1); res.OK || !res.hasCode(CodeMissingSection) {
		t.Fatalf("version 1 without the section must be a missing_section violation: %+v", res.Violations)
	}
	if res := verify(auditWithoutExplanation, 0); !res.OK {
		t.Fatalf("version 0 must not ask for the section: %+v", res.Violations)
	}
	if res := verify(auditWithExplanation, 1); !res.OK {
		t.Fatalf("the section present must pass: %+v", res.Violations)
	}
	for name, body := range map[string]string{
		"level-three heading": auditWithoutExplanation + "\n### Explanation Documentation\n- Status: VERIFIED\n",
		"fenced heading":      auditWithoutExplanation + "\n```\n## Explanation Documentation\n```\n",
	} {
		if res := verify(body, 1); res.OK {
			t.Errorf("%s must not count as the section (the audit gate would refuse it): %+v", name, res.Violations)
		}
	}
	// A missing artifact carries only its own violation.
	ws := t.TempDir()
	res, err := VerifyWithStage("audit", phasecontract.Roots{Workspace: ws, ExplanationDocumentationVersion: 1}, phasecontract.BuiltinResolver{}, config.StageOff)
	if err != nil || len(res.Violations) != 1 || !res.hasCode(CodeMissingArtifact) {
		t.Fatalf("missing artifact: %+v %v", res.Violations, err)
	}
	// Other phases declare no explanation sections.
	if c, _ := phasecontract.For("scout"); len(c.ExplanationSections) != 0 {
		t.Fatal("scout must not owe an explanation section")
	}
}
