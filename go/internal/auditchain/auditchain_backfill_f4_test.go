package auditchain

import (
	"strings"
	"testing"
)

func TestWithLink_AppendsAMissingLinkWithUnsetCitationAndNeverMutatesTheInput(t *testing.T) {
	base := Chain{{ID: LinkIntentFidelity, Status: StatusCoherent, Finding: "ok", Citation: "intent.md:1"}}

	got := withLink(base, LinkDelivery, StatusIncoherent, "delivers B")

	if len(got) != 2 || got[1] != (Link{ID: LinkDelivery, Status: StatusIncoherent, Finding: "delivers B", Citation: "unset"}) {
		t.Fatalf("appended link = %+v", got)
	}
	if len(base) != 1 || base[0].Status != StatusCoherent {
		t.Errorf("input chain must stay untouched: %+v", base)
	}
}

func TestWithLink_ReplacesAnExistingLinkInACopyKeepingItsCitation(t *testing.T) {
	base := Chain{{ID: LinkNarrative, Status: StatusCoherent, Finding: "ok", Citation: "build-report.md:3"}}

	got := withLink(base, LinkNarrative, StatusUnverifiable, "cannot tell")

	if len(got) != 1 || got[0].Status != StatusUnverifiable || got[0].Finding != "cannot tell" || got[0].Citation != "build-report.md:3" {
		t.Fatalf("replaced link = %+v", got)
	}
	if base[0].Status != StatusCoherent {
		t.Errorf("input chain was mutated: %+v", base)
	}
}

func TestParseChainBlock_RejectsUnclosedAndEmptyBlocks(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"no block", "# report\nno chain here\n", "no chain block in the report"},
		{"opened but never closed", chainBlockOpen + "\nintent-fidelity | coherent | intent.md:1 | ok\n", "is not closed"},
		{"closed with only blank lines", chainBlockOpen + "\n\n   \n" + chainBlockClose, "chain block is empty"},
		{"closed immediately", chainBlockOpen + chainBlockClose, "chain block is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ParseChainBlock(tc.content)
			if err == nil || !strings.Contains(err.Error(), tc.want) || c != nil {
				t.Fatalf("chain=%v err=%v, want %q", c, err, tc.want)
			}
		})
	}
}

func TestConcludeWithEvidence_IncoherentLinkIsNotDowngradedAndStillFails(t *testing.T) {
	c := withLink(fullChain(), LinkEvidence, StatusIncoherent, "gate output forged")
	c = withLink(c, LinkNarrative, StatusCoherent, "claims match")

	got := ConcludeWithEvidence(c, "audit", []string{"intent.md", "scout-report.md", "triage-decision.json", "covering-tests.md", "build-report.md", "coverage-gate-report.md"})

	if got.Verdict != VerdictFAIL {
		t.Fatalf("verdict = %s, want FAIL; rationale=%s", got.Verdict, got.Rationale)
	}
	if !strings.Contains(got.Rationale, "acs-verdict.json (no link lost every source; conclusion stands)") {
		t.Errorf("an incoherent link is never downgraded, so no link may be listed as downgraded: %s", got.Rationale)
	}
	if !strings.Contains(got.Rationale, "incoherent link(s): evidence-fidelity (gate output forged)") {
		t.Errorf("original incoherent finding must survive: %s", got.Rationale)
	}
}

func TestConcludeWithEvidence_MissingEvidenceNoLinkDependsOnKeepsTheConclusion(t *testing.T) {
	c := Chain{{ID: LinkIntentFidelity, Status: StatusCoherent, Finding: "ok", Citation: "intent.md:1"}}

	got := ConcludeWithEvidence(c, "audit", []string{"intent.md", "scout-report.md", "triage-decision.json", "covering-tests.md", "build-report.md", "coverage-gate-report.md"})

	want := Conclude(c)
	if got.Verdict != want.Verdict || len(got.Diagnoses) != len(want.Diagnoses) {
		t.Fatalf("verdict %s/%s, diagnoses %v/%v", got.Verdict, want.Verdict, got.Diagnoses, want.Diagnoses)
	}
	if !strings.HasPrefix(got.Rationale, "evidence not supplied to audit: acs-verdict.json (no link lost every source; conclusion stands). ") {
		t.Errorf("rationale = %s", got.Rationale)
	}
	if !strings.HasSuffix(got.Rationale, want.Rationale) {
		t.Errorf("rationale must keep the underlying conclusion: %s", got.Rationale)
	}
}
