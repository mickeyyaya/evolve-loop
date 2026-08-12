package auditchain

// auditchain_test.go — the audit verdict as the CONCLUSION of a chain of
// reasoning across every prior phase, rather than an assertion attached to one.
//
// WHY. A human reviewer does not compute a score from the diff. They hold the
// whole chain at once — what was asked, what was planned, what the tests
// demanded, what the builder claimed, what the bytes do — and they see
// immediately when a change is derailed, specious, paradoxical or deceptive.
// Those four are not properties of a diff. They are INCOHERENCES BETWEEN
// STAGES, and they are invisible to any check that reads one artifact alone.
//
// So the audit's obligation is not "produce a verdict". It is "produce the
// chain", and the verdict is a function of it. Judgement stays where judgement
// belongs (is this link coherent?); entailment is deterministic (given these
// link statuses, what verdict follows?) — an auditor cannot assert PASS over an
// incoherent link, because the verdict is not the auditor's to assert.

import (
	"strings"
	"testing"
)

func coherent(id LinkID) Link {
	return Link{ID: id, Status: StatusCoherent, Finding: "matches", Citation: "audit-report.md:12"}
}

func fullChain() Chain {
	var c Chain
	for _, id := range RequiredLinks() {
		c = append(c, coherent(id))
	}
	return c
}

// --- Entailment: the verdict is computed, never asserted -----------------

func TestConclude_VerdictFollowsFromTheChain(t *testing.T) {
	t.Parallel()
	if got := Conclude(fullChain()); got.Verdict != VerdictPASS {
		t.Errorf("a fully coherent chain entails PASS, got %s (%s)", got.Verdict, got.Rationale)
	}

	// One incoherent link is decisive, and the conclusion must NAME it: an
	// operator holding a FAIL needs to know which relationship broke, not that
	// something did.
	c := fullChain()
	c[3].Status = StatusIncoherent
	c[3].Finding = "the test was relaxed to match the implementation"
	got := Conclude(c)
	if got.Verdict != VerdictFAIL {
		t.Errorf("an incoherent link entails FAIL, got %s", got.Verdict)
	}
	if !strings.Contains(got.Rationale, string(c[3].ID)) {
		t.Errorf("the rationale must name the broken link; got %q", got.Rationale)
	}
}

// The honest middle. "I could not check this" is not evidence of coherence, and
// it must not be launderable into one — but it is also not a defect, so it
// cannot simply FAIL either.
func TestConclude_UnverifiableCannotSupportPASS(t *testing.T) {
	t.Parallel()
	c := fullChain()
	c[2].Status = StatusUnverifiable
	c[2].Finding = "tdd artifacts absent from the workspace"
	got := Conclude(c)
	if got.Verdict == VerdictPASS {
		t.Error("an unverifiable link supported a PASS — 'I could not check' became 'I checked and it was fine'")
	}
	if got.Verdict != VerdictWARN {
		t.Errorf("unverifiable entails WARN (resolve it or accept a qualified verdict), got %s", got.Verdict)
	}
	if !strings.Contains(got.Rationale, "unverifiable") {
		t.Errorf("the rationale must say what was not established; got %q", got.Rationale)
	}
}

// A chain missing a required link is not a chain. Silence about a relationship
// is the cheapest way to avoid reporting it, so absence must be louder than a
// negative finding, not quieter.
func TestConclude_AnIncompleteChainCannotConclude(t *testing.T) {
	t.Parallel()
	c := fullChain()[:len(RequiredLinks())-1]
	got := Conclude(c)
	if got.Verdict == VerdictPASS {
		t.Error("a chain with a missing link concluded PASS — omission became the cheapest bypass")
	}
	if !strings.Contains(got.Rationale, "missing") {
		t.Errorf("the rationale must name the absence; got %q", got.Rationale)
	}
}

// --- Citations: a link without one is an assertion ----------------------

func TestValidate_EveryLinkMustCiteSomethingCheckable(t *testing.T) {
	t.Parallel()
	c := fullChain()
	c[1].Citation = ""
	errs := Validate(c)
	if len(errs) == 0 {
		t.Error("a link with no citation was accepted — that is the auditor's opinion wearing the shape of a finding")
	}
	if !strings.Contains(errs[0].Error(), string(c[1].ID)) {
		t.Errorf("the error must name the link; got %v", errs[0])
	}
}

func TestValidate_RejectsDuplicateAndUnknownLinks(t *testing.T) {
	t.Parallel()
	c := append(fullChain(), coherent(LinkDelivery)) // duplicate
	if errs := Validate(c); len(errs) == 0 {
		t.Error("a duplicated link lets one relationship be reported twice with different statuses")
	}
	if errs := Validate(Chain{{ID: "invented", Status: StatusCoherent, Finding: "f", Citation: "c"}}); len(errs) == 0 {
		t.Error("an unknown link id was accepted — the chain's shape is the contract")
	}
}

// --- The four things a human sees immediately ---------------------------

// Each of derailed / specious / paradoxical / deceptive is a SPECIFIC pattern
// of link failures, which is exactly why a diff-only check cannot see them.
func TestDiagnose_NamesTheHumanRecognisableFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func(Chain) Chain
		want string
	}{
		{
			// The work is internally consistent and delivers something else.
			name: "derailed",
			mut: func(c Chain) Chain {
				return setLink(c, LinkDelivery, StatusIncoherent, "implements a cache; the intent asked for a retry budget")
			},
			want: "derailed",
		},
		{
			// The narrative is larger than the bytes.
			name: "specious",
			mut: func(c Chain) Chain {
				return setLink(c, LinkNarrative, StatusIncoherent, "build report claims a fix the diff does not contain")
			},
			want: "specious",
		},
		{
			// The implementation satisfies the tests because the tests were
			// moved to it. Each link looks fine alone; together they contradict.
			name: "paradoxical",
			mut: func(c Chain) Chain {
				c = setLink(c, LinkSpecification, StatusIncoherent, "acceptance criteria no longer encoded by the tests")
				return setLink(c, LinkImplementation, StatusCoherent, "implementation satisfies the tests as they now stand")
			},
			want: "paradoxical",
		},
		{
			// The evidence was produced by the party being judged.
			name: "deceptive",
			mut: func(c Chain) Chain {
				return setLink(c, LinkEvidence, StatusIncoherent, "the cited green run is the agent's own transcript, not an executed gate")
			},
			want: "deceptive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := strings.Join(Diagnose(tc.mut(fullChain())), " | ")
			if !strings.Contains(got, tc.want) {
				t.Errorf("Diagnose = %q, want it to name %q", got, tc.want)
			}
		})
	}

	// A coherent chain diagnoses nothing: the vocabulary must stay meaningful.
	if d := Diagnose(fullChain()); len(d) != 0 {
		t.Errorf("a coherent chain must diagnose nothing, got %v", d)
	}
}

// The paradox check is the one that cannot be done per-link, so it is the one
// worth pinning hardest: BOTH halves incoherent is an ordinary double failure,
// not a paradox.
func TestDiagnose_ParadoxRequiresTheContradiction(t *testing.T) {
	t.Parallel()
	c := setLink(fullChain(), LinkSpecification, StatusIncoherent, "criteria not encoded")
	c = setLink(c, LinkImplementation, StatusIncoherent, "and the code does not satisfy them either")
	if got := strings.Join(Diagnose(c), " "); strings.Contains(got, "paradox") {
		t.Errorf("two plain failures are not a paradox; got %q", got)
	}
}

// --- The audit must have read the prior phases at all -------------------

// The whole design rests on the auditor holding every prior stage. A chain
// whose citations all point at one artifact is an auditor that read one thing
// and inferred the rest.
func TestValidate_ChainMustCiteMoreThanOneStage(t *testing.T) {
	t.Parallel()
	c := fullChain()
	for i := range c {
		c[i].Citation = "audit-report.md:1"
	}
	errs := Validate(c)
	var found bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "single artifact") {
			found = true
		}
	}
	if !found {
		t.Error("every link cited the same artifact and the chain validated — the auditor read one file and inferred a chain it never walked")
	}
}
