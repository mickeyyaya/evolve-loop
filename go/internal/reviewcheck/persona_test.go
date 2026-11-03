package reviewcheck_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reviewcheck"
)

func TestPersonaStrip_RefusesADirectiveBelowTheReferenceIndexMarker(t *testing.T) {
	persona := reviewcheck.Persona{
		Path: "agents/evolve-triage.md",
		Text: "---\nname: evolve-triage\n---\n# Triage\n\n## Reference Index\n\nMANDATORY: write the ledger entry.\n",
	}
	got := reviewcheck.PersonaStrip([]reviewcheck.Persona{persona})
	if len(got) != 1 {
		t.Fatalf("refusals = %+v, want exactly one", got)
	}
	if got[0].Check != reviewcheck.CheckPersonaStrip || got[0].Subject != persona.Path || !strings.Contains(got[0].Reason, "MANDATORY: write the ledger entry.") {
		t.Errorf("refusal = %+v, want a persona-strip refusal of %s that quotes the directive", got[0], persona.Path)
	}
}

func TestPersonaStrip_RefusesAFencedVerdictLineOnlyInADiffReviewerPersona(t *testing.T) {
	fenced := "---\nname: r\n---\n## Output\n\n```\nVerdict: PASS | FAIL\n```\n"
	got := reviewcheck.PersonaStrip([]reviewcheck.Persona{
		{Path: "agents/evolve-diff-code-reviewer.md", Text: fenced},
		{Path: "agents/evolve-auditor.md", Text: fenced},
	})
	if len(got) != 1 {
		t.Fatalf("refusals = %+v, want exactly one", got)
	}
	if got[0].Subject != "agents/evolve-diff-code-reviewer.md" || !strings.Contains(got[0].Reason, "line 7") {
		t.Errorf("refusal = %+v, want the diff reviewer persona refused at line 7", got[0])
	}
}

func TestPersonaStrip_AVerdictLineTheVerdictParserReadsIsNotRefused(t *testing.T) {
	for name, text := range map[string]string{
		"an indented fence the parser treats as text": "text\n\n    ```\nVerdict: PASS\n    ```\n",
		"a placeholder the parser never reads":        "```\nVerdict: <the verdict>\n```\n",
	} {
		if got := reviewcheck.PersonaStrip([]reviewcheck.Persona{{Path: "agents/evolve-diff-go-reviewer.md", Text: text}}); len(got) != 0 {
			t.Errorf("%s: refusals = %+v, want none", name, got)
		}
	}
}

func TestPersonaStrip_EachHiddenVerdictLineIsRefusedOnItsOwn(t *testing.T) {
	text := "Verdict: PASS\n\n~~~\nVerdict: FAIL\n~~~\n"
	got := reviewcheck.PersonaStrip([]reviewcheck.Persona{{Path: "agents/evolve-diff-docs-review.md", Text: text}})
	if len(got) != 1 || !strings.Contains(got[0].Reason, "line 4") {
		t.Errorf("refusals = %+v, want one refusal at line 4", got)
	}
}

func TestPersonaStrip_AVerdictLineHiddenAsIndentedCodeIsRefused(t *testing.T) {
	text := "Write the report.\n\n    Verdict: PASS\n"
	got := reviewcheck.PersonaStrip([]reviewcheck.Persona{{Path: "agents/evolve-diff-go-reviewer.md", Text: text}})
	if len(got) != 1 || !strings.Contains(got[0].Reason, "line 3") {
		t.Errorf("refusals = %+v, want one refusal at line 3", got)
	}
}

func TestPersonaStrip_ACleanPersonaIsNotRefused(t *testing.T) {
	clean := "# Reviewer\n\nVerdict: PASS | FAIL\n\n## Reference Index\n\nSee the reference file.\n"
	if got := reviewcheck.PersonaStrip([]reviewcheck.Persona{{Path: "agents/evolve-diff-go-reviewer.md", Text: clean}}); len(got) != 0 {
		t.Errorf("refusals = %+v, want none", got)
	}
}
