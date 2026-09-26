package phasecontract

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

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

func TestExplanationDocumentation_TitleAndProducerDeclaration(t *testing.T) {
	if ExplanationDocumentation.Title() != "Explanation Documentation" {
		t.Fatalf("Title() = %q", ExplanationDocumentation.Title())
	}
	union := producerUnion(t, agentsDir(t), Audit.Producers)
	if !strings.Contains(union, ExplanationDocumentation.Canonical) {
		t.Fatalf("the audit producers %v do not declare %q — template/contract drift", Audit.Producers, ExplanationDocumentation.Canonical)
	}
}
