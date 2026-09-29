package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const reauthorDocument = "docs/explain/builds/cycle-1745-01m3mak46kk0havqvsbxmhaq4q.md"

func TestComposePrompt_AnExplanationReauthorScopesTheRoundToTheDocument(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{
		core.CtxKeyExplanationReauthor: reauthorDocument,
		core.CtxKeyAuditRepairFindings: "failed phase: audit\n- explanation-needs-correction: " + reauthorDocument + ":8 states 9 lines; the scanner measures 8",
	}}

	out := hooks{}.ComposePrompt("BODY", req)

	section := strings.Index(out, "## Explanation Re-author")
	repair := strings.Index(out, "## Audit Repair")
	if section < 0 || repair < 0 || section > repair {
		t.Fatalf("the re-author scope must precede the repair findings it scopes (section %d, repair %d):\n%s", section, repair, out)
	}
	scope := out[section:repair]
	for _, want := range []string{"Edit only " + reauthorDocument, "rewrite `build-report.md`", "the rewrite completes this dispatch", "leave code and tests unchanged", "The audit re-runs on the corrected document"} {
		if !strings.Contains(scope, want) {
			t.Errorf("the re-author section lacks %q:\n%s", want, scope)
		}
	}
}

func TestComposePrompt_NoExplanationReauthorLeavesTheRepairRoundWhole(t *testing.T) {
	req := core.PhaseRequest{Context: map[string]string{core.CtxKeyAuditRepairFindings: auditRepairFixtureReason}}

	out := hooks{}.ComposePrompt("BODY", req)

	if strings.Contains(out, "Explanation Re-author") {
		t.Errorf("a repair round with a defect outside the document must not be scoped to it:\n%s", out)
	}
}
