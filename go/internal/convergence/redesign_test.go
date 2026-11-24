package convergence_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestDecide_BlockCountEscalatesToRedesignThenToOperator(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane, []convergence.Finding{finding("h1", convergence.SeverityHigh)})
	base := decide(t, in)

	for _, redesigns := range []int{0, 1} {
		atThree := in
		atThree.Blocks, atThree.Redesigns = 3, redesigns
		if got := decide(t, atThree); !reflect.DeepEqual(got, base) {
			t.Fatalf("at 3 blocks and %d redesigns: got %+v, want %+v", redesigns, got, base)
		}
	}

	first := in
	first.Blocks = 4
	dFirst := decide(t, first)
	requireOutcome(t, dFirst, convergence.ActionRedesign, 3)
	requireReason(t, dFirst, "CONVERGENCE_RUNG", "action=Redesign", "cause=blocks")
	if !reflect.DeepEqual(dFirst.Redesign, base.Redesign) {
		t.Fatalf("Decision.Redesign changed from %q to %q", base.Redesign, dFirst.Redesign)
	}

	second := in
	second.Blocks, second.Redesigns = 4, 1
	dSecond := decide(t, second)
	requireOutcome(t, dSecond, convergence.ActionToOperator, 3)
	requireReason(t, dSecond, "CONVERGENCE_RUNG", "action=ToOperator", "cause=blocks")
}

func TestDecide_CleanRoundAtFourBlocksStillLands(t *testing.T) {
	in := inputFor(convergence.LoopCodeReview, nil)
	base := decide(t, in)
	in.Blocks, in.Redesigns = 4, 1
	if got := decide(t, in); !reflect.DeepEqual(got, base) {
		t.Fatalf("clean round at 4 blocks: got %+v, want %+v", got, base)
	}
}

func TestParse_SubjectRoundTripsAndHostCountsAreRefused(t *testing.T) {
	in, err := convergence.Parse([]byte(`{"loop":"console-lane","subject":"item-1","round":0,"rounds":[{"index":0,"findings":[]}]}`))
	if err != nil {
		t.Fatalf("Parse subject: %v", err)
	}
	if in.Subject != "item-1" || in.Blocks != 0 || in.Redesigns != 0 {
		t.Fatalf("parsed %+v, want Subject=item-1 and zero counts", in)
	}
	for _, forged := range []string{
		`{"loop":"console-lane","round":0,"blocks":0,"rounds":[]}`,
		`{"loop":"console-lane","round":0,"redesigns":0,"rounds":[]}`,
	} {
		if _, err := convergence.Parse([]byte(forged)); err == nil {
			t.Errorf("Parse(%s) = nil, want unknown-field refusal", forged)
		}
	}
}
