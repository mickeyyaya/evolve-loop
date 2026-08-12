package auditchain

// evidence_access_test.go — the precondition the chain rests on: a phase that
// JUDGES must be handed what the phases before it produced.
//
// A reviewer asked to rule on a diff alone cannot see derailment (needs the
// intent), speciousness (needs the build report), the paradox (needs the tests
// AND the criteria they were supposed to encode), or deception (needs the gate
// outputs). Withholding those artifacts does not make the audit stricter — it
// makes it guess, and a guess that must produce a verdict produces PASS.
//
// So entitlement is a CONTRACT, not a convenience: every judging phase declares
// what it must be able to read, and a dispatch that omits one is a pipeline
// defect rather than a thinner prompt.

import (
	"strings"
	"testing"
)

func TestJudgingPhases_CoverEveryPhaseThatRulesOnAnother(t *testing.T) {
	t.Parallel()
	// Every phase whose output is a judgement about work someone else did.
	for _, p := range []string{"audit", "adversarial-review", "coverage-gate", "plan-review", "inherited-defect-reconcile", "retrospective"} {
		if !IsJudging(p) {
			t.Errorf("%q rules on another phase's work and must be treated as judging — otherwise it is asked for a verdict without the evidence for one", p)
		}
	}
	// Producing phases are not judging: handing them the full prior corpus
	// would be cost with no decision attached.
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
	// The chain is only walkable if every required link has an artifact behind
	// it. This is the property that makes the two designs one design.
	for _, id := range RequiredLinks() {
		if _, ok := EvidenceFor(id); !ok {
			t.Errorf("link %s has no declared evidence source — an auditor could only assert it", id)
		}
	}
	// And everything a link needs must actually be in the entitlement.
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

// A non-judging phase gets nothing extra: the entitlement must not become a
// blanket "everyone reads everything", which is how prompts grow until they are
// truncated — this repo lost 15 of 30 cycles to a prompt that was silently cut.
func TestRequiredEvidence_IsScopedToJudgingPhases(t *testing.T) {
	t.Parallel()
	if got := RequiredEvidence("build"); len(got) != 0 {
		t.Errorf("a producing phase was handed the judging corpus: %v", got)
	}
}

// The dispatch check: what a judging phase was ACTUALLY given, versus what it
// was entitled to. A missing artifact is named, because the failure it causes
// downstream (an unverifiable link, or worse an asserted one) is invisible at
// the point it happens.
func TestMissingEvidence_NamesWhatTheJudgeWasNotGiven(t *testing.T) {
	t.Parallel()
	given := []string{"build-report.md", "acs-verdict.json"}
	missing := MissingEvidence("audit", given)
	if len(missing) == 0 {
		t.Fatal("a judging phase dispatched without the intent or the tests reported no gap")
	}
	joined := strings.Join(missing, ",")
	if !strings.Contains(joined, "intent") {
		t.Errorf("the intent is what derailment is measured against; its absence must be named. got %v", missing)
	}
	// Fully supplied: nothing missing.
	if m := MissingEvidence("audit", RequiredEvidence("audit")); len(m) != 0 {
		t.Errorf("a fully-supplied judge reported gaps: %v", m)
	}
	// Not judging: never a gap, whatever it was given.
	if m := MissingEvidence("build", nil); len(m) != 0 {
		t.Errorf("a producing phase cannot be short of judging evidence: %v", m)
	}
}

// An unread entitlement is worth nothing, so the absence has to reach the
// verdict rather than sit in a log: a chain assembled without the evidence for
// a link cannot report that link as coherent.
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
	// Fully supplied and coherent still passes — the check must not become a
	// second, unconditional blocker.
	if got := ConcludeWithEvidence(c, "audit", RequiredEvidence("audit")); got.Verdict != VerdictPASS {
		t.Errorf("a fully-supplied coherent chain must PASS, got %s (%s)", got.Verdict, got.Rationale)
	}
}

// TestChainShape_NamesEveryLinkAndItsConclusionType pins the two links the
// behavioural suite reaches only through RequiredLinks(), and the Conclusion
// type the caller destructures. Not ceremony: the first two links are where a
// task gets quietly restated into something easier, which is the failure that
// leaves no trace in the diff at all.
func TestChainShape_NamesEveryLinkAndItsConclusionType(t *testing.T) {
	t.Parallel()
	// LinkIntentFidelity is the only check standing between "the item asked for
	// X" and an intent that asked for something smaller.
	srcs, ok := EvidenceFor(LinkIntentFidelity)
	if !ok || len(srcs) == 0 {
		t.Error("intent-fidelity must be read from an artifact — a restated task is invisible in the diff")
	}
	// LinkSelection catches the cycle that solved a different queued item.
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
