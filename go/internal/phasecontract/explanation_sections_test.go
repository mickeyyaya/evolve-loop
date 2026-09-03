package phasecontract

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// TestAuditContract_ExplanationSectionIsConditional pins the declaration the
// deliverable reviewer projects: the audit report owes "## Explanation
// Documentation" only while the explanation contract is active, so it must
// live under ExplanationSections and never under the always-on Sections.
func TestAuditContract_ExplanationSectionIsConditional(t *testing.T) {
	t.Parallel()
	c, ok := For("audit")
	if !ok {
		t.Fatal("audit contract missing")
	}
	if len(c.ExplanationSections) != 1 || c.ExplanationSections[0].Canonical != ExplanationDocumentation.Canonical {
		t.Fatalf("audit ExplanationSections = %+v, want the ExplanationDocumentation section", c.ExplanationSections)
	}
	for _, s := range c.Sections {
		if s.Canonical == ExplanationDocumentation.Canonical {
			t.Fatal("the explanation section must not be an always-on audit section (cycles without the contract would be asked for it)")
		}
	}
	if !reportdoc.HasSection("# Audit\n\n## Explanation Documentation\n- Status: VERIFIED\n", ExplanationDocumentation.Title()) || reportdoc.HasSection("## Verdict\nPASS\n### Explanation Documentation\n", ExplanationDocumentation.Title()) {
		t.Fatal("the production predicate (reportdoc.HasSection over Title) must match the exact level-two heading and only that")
	}
}

// TestExplanationDocumentation_TitleAndProducerDeclaration — the heading the
// deliverable gate matches (Title, exact) is the one the auditor's reference
// template declares, so a template rename fails here, not in a live cycle.
func TestExplanationDocumentation_TitleAndProducerDeclaration(t *testing.T) {
	if ExplanationDocumentation.Title() != "Explanation Documentation" {
		t.Fatalf("Title() = %q", ExplanationDocumentation.Title())
	}
	union := producerUnion(t, agentsDir(t), Audit.Producers)
	if !strings.Contains(union, ExplanationDocumentation.Canonical) {
		t.Fatalf("the audit producers %v do not declare %q — template/contract drift", Audit.Producers, ExplanationDocumentation.Canonical)
	}
}
