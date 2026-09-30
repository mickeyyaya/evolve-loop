package auditchain

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

func TestConclude_VerdictFollowsFromTheChain(t *testing.T) {
	t.Parallel()
	if got := Conclude(fullChain()); got.Verdict != VerdictPASS {
		t.Errorf("a fully coherent chain entails PASS, got %s (%s)", got.Verdict, got.Rationale)
	}

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
	duplicated := append(fullChain(), coherent(LinkDelivery))
	if errs := Validate(duplicated); len(errs) == 0 {
		t.Error("a duplicated link lets one relationship be reported twice with different statuses")
	}
	if errs := Validate(Chain{{ID: "invented", Status: StatusCoherent, Finding: "f", Citation: "c"}}); len(errs) == 0 {
		t.Error("an unknown link id was accepted — the chain's shape is the contract")
	}
}

func TestDiagnose_NamesTheHumanRecognisableFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mut  func(Chain) Chain
		want string
	}{
		{
			name: "derailed",
			mut: func(c Chain) Chain {
				return withLink(c, LinkDelivery, StatusIncoherent, "implements a cache; the intent asked for a retry budget")
			},
			want: "derailed",
		},
		{
			name: "specious",
			mut: func(c Chain) Chain {
				return withLink(c, LinkNarrative, StatusIncoherent, "build report claims a fix the diff does not contain")
			},
			want: "specious",
		},
		{
			name: "paradoxical",
			mut: func(c Chain) Chain {
				c = withLink(c, LinkSpecification, StatusIncoherent, "acceptance criteria no longer encoded by the tests")
				return withLink(c, LinkImplementation, StatusCoherent, "implementation satisfies the tests as they now stand")
			},
			want: "paradoxical",
		},
		{
			name: "deceptive",
			mut: func(c Chain) Chain {
				return withLink(c, LinkEvidence, StatusIncoherent, "the cited green run is the agent's own transcript, not an executed gate")
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

	if d := Diagnose(fullChain()); len(d) != 0 {
		t.Errorf("a coherent chain must diagnose nothing, got %v", d)
	}
}

func TestDiagnose_ParadoxRequiresTheContradiction(t *testing.T) {
	t.Parallel()
	c := withLink(fullChain(), LinkSpecification, StatusIncoherent, "criteria not encoded")
	c = withLink(c, LinkImplementation, StatusIncoherent, "and the code does not satisfy them either")
	if got := strings.Join(Diagnose(c), " "); strings.Contains(got, "paradox") {
		t.Errorf("two plain failures are not a paradox; got %q", got)
	}
}

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
