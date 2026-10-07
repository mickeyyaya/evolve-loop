package convergence_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func afterOneFix(loop convergence.Loop, f convergence.Finding) convergence.Decision {
	in := inputFor(loop,
		many("a", 2, convergence.SeverityCritical),
		join(asFixed(many("a", 2, convergence.SeverityCritical)), []convergence.Finding{f}))
	return convergence.Decide(in)
}

func TestLate_BelowHighIsFiledAndNeverBlocks(t *testing.T) {
	for _, f := range []convergence.Finding{
		finding("x", convergence.SeverityMedium, late, survived), finding("x", convergence.SeverityMedium, late),
		finding("x", convergence.SeverityLow, late, survived), finding("x", convergence.SeverityLow, late),
	} {
		d := afterOneFix(convergence.LoopConsoleLane, f)

		requireOutcome(t, d, convergence.ActionLand, 0)
		if !reflect.DeepEqual(d.File, []string{"x"}) {
			t.Fatalf("late %s (falsification %q): file %q, want it filed", f.Severity, f.Falsification, d.File)
		}
		requireReason(t, d, "CONVERGENCE_FILED", "late=1", "capability=0")
	}
}

func TestLate_ACriticalOrHighIsFiledOnlyWhenItsFalsificationRefutedIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		f        convergence.Finding
		blocks   bool
		wantFile []string
	}{
		{"late CRITICAL, certificate survived", finding("x", convergence.SeverityCritical, late, survived), true, nil},
		{"late CRITICAL, certificate refuted", finding("x", convergence.SeverityCritical, late, refuted), false, []string{"x"}},
		{"late CRITICAL, falsification not run", finding("x", convergence.SeverityCritical, late), true, nil},
		{"late HIGH, certificate survived", finding("x", convergence.SeverityHigh, late, survived), true, nil},
		{"late HIGH, certificate refuted", finding("x", convergence.SeverityHigh, late, refuted), false, []string{"x"}},
		{"late HIGH, falsification not run", finding("x", convergence.SeverityHigh, late), true, nil},
		{"not late HIGH, certificate refuted", finding("x", convergence.SeverityHigh, refuted), true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := afterOneFix(convergence.LoopConsoleLane, tc.f)

			wantAction, wantRung := convergence.ActionLand, 0
			if tc.blocks {
				wantAction, wantRung = convergence.ActionContinue, 1
			}
			requireOutcome(t, d, wantAction, wantRung)
			if !reflect.DeepEqual(d.File, tc.wantFile) {
				t.Fatalf("file %q, want %q", d.File, tc.wantFile)
			}
		})
	}
}

func TestLate_ACriticalOrHighWhoseFalsificationNeverRanBlocks(t *testing.T) {
	for _, sev := range []convergence.Severity{convergence.SeverityCritical, convergence.SeverityHigh} {
		d := decide(t, inputFor(convergence.LoopCodeReview, []convergence.Finding{finding("x", sev, late)}))

		requireOutcome(t, d, convergence.ActionContinue, 0)
		if len(d.File) != 0 {
			t.Fatalf("late %s with no falsification result: file %q, want it blocking until the check runs", sev, d.File)
		}
	}
}

func TestCapability_BelowCriticalIsFiledAtOnceAndACriticalIsADefect(t *testing.T) {
	high := afterOneFix(convergence.LoopConsoleLane, finding("cap", convergence.SeverityHigh, capability))

	requireOutcome(t, high, convergence.ActionLand, 0)
	if !reflect.DeepEqual(high.File, []string{"cap"}) {
		t.Fatalf("capability HIGH: file %q, want it filed", high.File)
	}
	requireReason(t, high, "CONVERGENCE_FILED", "late=0", "capability=1")

	critical := afterOneFix(convergence.LoopConsoleLane, finding("cap", convergence.SeverityCritical, capability))

	requireOutcome(t, critical, convergence.ActionContinue, 1)
	if len(critical.File) != 0 {
		t.Fatalf("capability CRITICAL: file %q, want it blocking as a defect, never filed", critical.File)
	}
}

func TestCapability_IsFiledAtEveryRung(t *testing.T) {
	in := inputOf(convergence.LoopCodeReview, rung2Rounds())
	in.Rounds[2].Findings = append(in.Rounds[2].Findings, finding("cap", convergence.SeverityMedium, capability))

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionContinue, 2)
	if !reflect.DeepEqual(d.File, []string{"i2", "cap"}) || !reflect.DeepEqual(d.Defer, []string{"m2", "l2"}) {
		t.Fatalf("file %q defer %q, want the capability filed, never deferred", d.File, d.Defer)
	}
}

func TestAuditRepair_RecordsKindAndLateButNeverUsesThemToUnblock(t *testing.T) {
	for _, f := range []convergence.Finding{
		finding("x", convergence.SeverityHigh, late),
		finding("x", convergence.SeverityHigh, capability),
	} {
		d := afterOneFix(convergence.LoopAuditRepair, f)

		requireOutcome(t, d, convergence.ActionContinue, 1)
		if len(d.File) != 0 || len(d.Defer) != 0 {
			t.Fatalf("audit finding %+v: file %q defer %q, want it blocking", f, d.File, d.Defer)
		}
	}
}

func TestDisputed_NeverBlocksAndIsLeftToTheAdjudicator(t *testing.T) {
	d := afterOneFix(convergence.LoopCodeReview, finding("x", convergence.SeverityCritical, disputed))

	requireOutcome(t, d, convergence.ActionLand, 0)
	if len(d.File) != 0 || len(d.Defer) != 0 {
		t.Fatalf("file %q defer %q, want the disputed row left to the adjudicator", d.File, d.Defer)
	}
}

func TestKeepBest_ALaterWorseRoundDoesNotLand(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("a", convergence.SeverityHigh), finding("b", convergence.SeverityHigh), finding("m", convergence.SeverityMedium)},
		[]convergence.Finding{finding("a", convergence.SeverityHigh, fixed), finding("b", convergence.SeverityHigh, fixed), finding("m", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m", convergence.SeverityMedium, fixed),
			finding("m2", convergence.SeverityMedium, at("go/x/x.go:3")), finding("m3", convergence.SeverityMedium, at("go/x/x.go:4"))})
	js[2].FixHunks = []convergence.Hunk{{File: "go/x/x.go", From: 1, To: 9}}

	d := decide(t, inputOf(convergence.LoopConsoleLane, js))

	requireOutcome(t, d, convergence.ActionLand, 2)
	if d.LandRound != 1 || !reflect.DeepEqual(d.Defer, []string{"m"}) {
		t.Fatalf("land round %d defer %q, want round 1 (tied with round 2 at the bar, but with less open mass) with its own MEDIUM deferred", d.LandRound, d.Defer)
	}
}

func TestKeepBest_ATieAtTheBarGoesToTheLowestOpenMassBeforeTheEarliestRound(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("m1", convergence.SeverityMedium), finding("m2", convergence.SeverityMedium), finding("m3", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m1", convergence.SeverityMedium, fixed), finding("m2", convergence.SeverityMedium, fixed), finding("m3", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m3", convergence.SeverityMedium)})

	d := decide(t, inputOf(convergence.LoopCodeReview, js))

	requireOutcome(t, d, convergence.ActionLand, 2)
	if d.LandRound != 1 || !reflect.DeepEqual(d.Defer, []string{"m3"}) {
		t.Fatalf("land round %d defer %q, want round 1 (one open MEDIUM against round 0's three, and earlier than round 2's one) with only m3 deferred", d.LandRound, d.Defer)
	}
}

func TestKeepBest_FewerStrictFindingsOutrankALighterResidue(t *testing.T) {
	for _, sev := range []convergence.Severity{convergence.SeverityHigh, convergence.SeverityCritical} {
		in := inputFor(convergence.LoopCodeReview,
			[]convergence.Finding{finding("s", sev)},
			join([]convergence.Finding{finding("s", sev, fixed)}, many("l", 9, convergence.SeverityLow)))

		d := decide(t, in)

		if d.Action != convergence.ActionLand || d.LandRound != 1 {
			t.Errorf("%s: %s of round %d, want Land of round 1: round 0 holds the open %s, round 1 only nine LOWs", sev, d.Action, d.LandRound, sev)
		}
	}
}

func TestKeepBest_AFullTieGoesToTheEarliestRound(t *testing.T) {
	js := judgments(
		[]convergence.Finding{finding("m1", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m1", convergence.SeverityMedium, fixed), finding("m2", convergence.SeverityMedium)},
		[]convergence.Finding{finding("m2", convergence.SeverityMedium, fixed), finding("m3", convergence.SeverityMedium)})

	d := decide(t, inputOf(convergence.LoopCodeReview, js))

	requireOutcome(t, d, convergence.ActionLand, 2)
	if d.LandRound != 0 || !reflect.DeepEqual(d.Defer, []string{"m1"}) {
		t.Fatalf("land round %d defer %q, want round 0: every round holds one open MEDIUM of the same mass", d.LandRound, d.Defer)
	}
}

func TestKeepBest_TheFewestOpenFindingsAtTheBarIsKeptOnAStop(t *testing.T) {
	in := inputFor(convergence.LoopConsoleLane,
		many("a", 3, convergence.SeverityHigh),
		join(asFixed(many("a", 2, convergence.SeverityHigh)), many("a", 3, convergence.SeverityHigh)[2:]),
		join(asFixed(many("a", 3, convergence.SeverityHigh)), many("b", 2, convergence.SeverityHigh)))
	in.Config = withBudget(1)

	d := decide(t, in)

	requireOutcome(t, d, convergence.ActionStop, 3)
	if d.LandRound != 1 {
		t.Fatalf("land round %d, want round 1 (1 open HIGH, against 3 and 2)", d.LandRound)
	}
	requireReason(t, d, "CONVERGENCE_STOP", "open=1", "land_round=1")
	if !reflect.DeepEqual(d.File, []string{"ac"}) {
		t.Fatalf("file %q, want the kept round's open finding filed", d.File)
	}
}
