package auditchain

import (
	"strings"
	"testing"
)

func TestJudgingPhases_CoverEveryPhaseThatRulesOnAnother(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"audit", "adversarial-review", "coverage-gate", "plan-review", "inherited-defect-reconcile", "retrospective"} {
		if !IsJudging(p) {
			t.Errorf("%q rules on another phase's work and must be treated as judging — otherwise it is asked for a verdict without the evidence for one", p)
		}
	}
	for _, p := range []string{"scout", "tdd", "build", "ship", "memo"} {
		if IsJudging(p) {
			t.Errorf("%q produces work rather than ruling on it; entitlement is about deciding, not about seniority", p)
		}
	}
}

func TestRequiredEvidence_EachLinkHasSomethingToReadItFrom(t *testing.T) {
	t.Parallel()
	ev := RequiredEvidence("audit")
	if len(ev) == 0 {
		t.Fatal("the audit was entitled to nothing — the chain would have to be narrated rather than walked")
	}
	for _, id := range RequiredLinks() {
		if _, ok := EvidenceFor(id); !ok {
			t.Errorf("link %s has no declared evidence source — an auditor could only assert it", id)
		}
	}
	entitled := map[string]bool{}
	for _, a := range ev {
		entitled[a] = true
	}
	for _, id := range RequiredLinks() {
		srcs, _ := EvidenceFor(id)
		for _, s := range srcs {
			if !entitled[s] {
				t.Errorf("link %s needs %q, which the audit is not entitled to read", id, s)
			}
		}
	}
}

func TestRequiredEvidence_IsScopedToJudgingPhases(t *testing.T) {
	t.Parallel()
	if got := RequiredEvidence("build"); len(got) != 0 {
		t.Errorf("a producing phase was handed the judging corpus: %v", got)
	}
}

func TestMissingEvidence_NamesWhatTheJudgeWasNotGiven(t *testing.T) {
	t.Parallel()
	given := []string{"build-report.md", "acs-verdict.json"}
	missing := MissingEvidence("audit", given)
	if len(missing) == 0 {
		t.Fatal("a judging phase dispatched without the tests or the triage decision reported no gap")
	}
	joined := strings.Join(missing, ",")
	if !strings.Contains(joined, "covering-tests.md") {
		t.Errorf("a withheld required artifact must be named; got %v", missing)
	}
	if strings.Contains(joined, "intent.md") {
		t.Errorf("a conditionally-produced artifact must not be reported as a withholding; got %v", missing)
	}
	if m := MissingEvidence("audit", RequiredEvidence("audit")); len(m) != 0 {
		t.Errorf("a fully-supplied judge reported gaps: %v", m)
	}
	if m := MissingEvidence("build", nil); len(m) != 0 {
		t.Errorf("a producing phase cannot be short of judging evidence: %v", m)
	}
}

func TestConcludeWithEvidence_UnsuppliedLinksCannotBeCoherent(t *testing.T) {
	t.Parallel()
	c := fullChain()
	got := ConcludeWithEvidence(c, "audit", []string{"build-report.md"})
	if got.Verdict == VerdictPASS {
		t.Error("a chain claimed every link coherent while the judge was never given the artifacts most of them are read from — that is a narrated chain and it must not PASS")
	}
	if !strings.Contains(got.Rationale, "not supplied") {
		t.Errorf("the rationale must say the evidence was missing, not merely that something failed; got %q", got.Rationale)
	}
	if got := ConcludeWithEvidence(c, "audit", RequiredEvidence("audit")); got.Verdict != VerdictPASS {
		t.Errorf("a fully-supplied coherent chain must PASS, got %s (%s)", got.Verdict, got.Rationale)
	}
}

func TestChainShape_NamesEveryLinkAndItsConclusionType(t *testing.T) {
	t.Parallel()
	srcs, ok := EvidenceFor(LinkIntentFidelity)
	if !ok || len(srcs) == 0 {
		t.Error("intent-fidelity must be read from an artifact — a restated task is invisible in the diff")
	}
	if srcs, ok := EvidenceFor(LinkSelection); !ok || len(srcs) == 0 {
		t.Error("selection-fidelity must be read from the triage decision")
	}
	var got Conclusion = Conclude(fullChain())
	if got.Verdict != VerdictPASS || got.Rationale == "" {
		t.Errorf("Conclusion must carry both the verdict and the reasoning that entails it, got %+v", got)
	}
	if got.Diagnoses != nil {
		t.Errorf("a coherent chain diagnoses nothing, got %v", got.Diagnoses)
	}
}

func TestConcludeWithEvidence_ALinkWithASurvivingSourceIsNotDowngraded(t *testing.T) {
	t.Parallel()
	var given []string
	for _, a := range RequiredEvidence("audit") {
		if a != "intent.md" {
			given = append(given, a)
		}
	}
	got := ConcludeWithEvidence(fullChain(), "audit", given)
	if got.Verdict != VerdictPASS {
		t.Errorf("a coherent chain was downgraded to %s because one of a link's ALTERNATIVE sources was absent: %s", got.Verdict, got.Rationale)
	}

	var blind []string
	for _, a := range RequiredEvidence("audit") {
		if a != "acs-verdict.json" && a != "coverage-gate-report.md" {
			blind = append(blind, a)
		}
	}
	if got := ConcludeWithEvidence(fullChain(), "audit", blind); got.Verdict == VerdictPASS {
		t.Error("evidence-fidelity passed with NEITHER of its sources supplied — the entitlement has no teeth left")
	}
}
