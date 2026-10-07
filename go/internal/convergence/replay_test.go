package convergence_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func loadL2(t *testing.T) convergence.Input {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "l2-rounds.json"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := convergence.Parse(data)
	if err != nil {
		t.Fatalf("Parse l2-rounds.json: %v", err)
	}
	in.Config, in.Headroom = defaults(), todaysTables
	return in
}

func prefix(in convergence.Input, r int) convergence.Input {
	in.Rounds, in.Round = in.Rounds[:r+1], r
	return in
}

func TestReplay_L2LandsAtRoundThreeThroughRungTwo(t *testing.T) {
	d := decide(t, loadL2(t))

	want := convergence.Decision{
		Rung: 2, Action: convergence.ActionLand, BlockingBar: convergence.SeverityHigh, LandRound: 3,
		Defer:    []string{"r7-MEDIUM-1", "r7-LOW-1", "r7-LOW-2", "r7-LOW-3", "r7-LOW-4"},
		File:     []string{"r7-INFO"},
		Redesign: []string{"bridge:model-check"},
	}
	got := d
	got.Reasons = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("L2 decision\n got %+v\nwant %+v\nreasons %q", got, want, d.Reasons)
	}
	requireReason(t, d, "CONVERGENCE_RUNG", "loop=console-lane", "round=3", "rung=2", "bar=HIGH", "action=Land")
	requireReason(t, d, "CONVERGENCE_CONCENTRATION", "component=bridge:model-check", "share=0.647", "prev_share=0.636", "window=2")
	requireReason(t, d, "CONVERGENCE_DEFERRED", "count=5")
	requireReason(t, d, "CONVERGENCE_FILED", "late=0", "capability=0", "info=1")
	refuseReason(t, d, "CONVERGENCE_NO_PROGRESS")
	refuseReason(t, d, "CONVERGENCE_REPAIR_DAMAGE")
}

func TestReplay_L2VerifiesFromJ1AndClimbsOneRungPerRoundBeforeConcentrationFires(t *testing.T) {
	l2 := loadL2(t)

	j0 := decide(t, prefix(l2, 0))
	requireOutcome(t, j0, convergence.ActionContinue, 0)
	if !j0.VerifyOnly || j0.BlockingBar != convergence.SeverityMedium {
		t.Fatalf("after J_0: %+v, want a verify-only fix at the MEDIUM bar", j0)
	}

	j1 := decide(t, prefix(l2, 1))
	requireOutcome(t, j1, convergence.ActionContinue, 1)
	requireReason(t, j1, "CONVERGENCE_NO_HEADROOM", "role=judge", "family=claude-tmux", "from=deep", "to=top")
	requireReason(t, j1, "CONVERGENCE_NO_HEADROOM", "role=fixer", "family=claude-tmux", "from=deep", "to=top")
	refuseReason(t, j1, "CONVERGENCE_CONCENTRATION")

	j2 := decide(t, prefix(l2, 2))
	requireOutcome(t, j2, convergence.ActionContinue, 2)
	wantDefer := []string{"r6-M1", "r6-M2", "r6-M3", "r6-L1", "r6-L2", "r6-L3", "r6-L4", "r6-L5", "r6-L6", "r6-L7"}
	if !j2.FreshContext || j2.BlockingBar != convergence.SeverityHigh || !reflect.DeepEqual(j2.Defer, wantDefer) {
		t.Fatalf("after J_2: %+v, want the fresh-context final round at HIGH with every MEDIUM and LOW deferred", j2)
	}
	refuseReason(t, j2, "CONVERGENCE_CONCENTRATION")
}

func auditRepairRounds() []convergence.Judgment {
	return judgments(
		[]convergence.Finding{finding("a1", convergence.SeverityHigh, ofClass("correctness")),
			finding("a2", convergence.SeverityMedium, ofClass("hygiene")), finding("a3", convergence.SeverityLow)},
		[]convergence.Finding{finding("a1", convergence.SeverityHigh, ofClass("correctness")), finding("a2", convergence.SeverityMedium, fixed),
			finding("a3", convergence.SeverityLow), finding("b1", convergence.SeverityMedium, ofClass("correctness"))},
		[]convergence.Finding{finding("a1", convergence.SeverityHigh, ofClass("correctness")), finding("b1", convergence.SeverityMedium, fixed),
			finding("a3", convergence.SeverityLow)})
}

func TestReplay_AuditRepairWithTheEnvelopeBudgetOfTwo(t *testing.T) {
	in := inputOf(convergence.LoopAuditRepair, auditRepairRounds())
	in.Config = withBudget(2)
	in.Fixer, in.Judge, in.Headroom = deepJudge, deepJudge, v3bTables

	final := decide(t, prefix(in, 1))

	requireOutcome(t, final, convergence.ActionContinue, 2)
	if !final.FreshContext || final.JudgeRaise != topJudge || final.FixerRaise != noRaise || final.VerifyOnly ||
		final.BlockingBar != convergence.SeverityMedium {
		t.Fatalf("round 2: %+v, want the fresh-context fixer with the judge effort raise, the audit's own bar, no verify-only and no second fixer raise", final)
	}

	stop := decide(t, in)

	requireOutcome(t, stop, convergence.ActionStop, 3)
	if !reflect.DeepEqual(stop.File, []string{"a1", "a3"}) || stop.LandRound != 2 {
		t.Fatalf("after round 2: %+v, want a Stop filing a1 and a3 with round 2 kept", stop)
	}
	for r := range in.Rounds {
		if d := decide(t, prefix(in, r)); len(d.Defer) != 0 {
			t.Fatalf("after round %d: defer %q, want no audit finding ever deferred", r, d.Defer)
		}
	}

	noHeadroom := prefix(in, 1)
	noHeadroom.Headroom = todaysTables
	requireReason(t, decide(t, noHeadroom), "CONVERGENCE_NO_HEADROOM", "loop=audit-repair", "role=judge")
}

func cycleRounds(edges ...string) []convergence.Judgment {
	js := steadyRounds(len(edges) - 1)
	for k, edge := range edges {
		js[k].Fingerprint, js[k].Edge = "fp-audit-red-ac3", edge
	}
	return js
}

func firstStop(t *testing.T, in convergence.Input) int {
	t.Helper()
	for r := range in.Rounds {
		d := decide(t, prefix(in, r))
		if d.Action != convergence.ActionContinue {
			requireOutcome(t, d, convergence.ActionStop, 3)
			return r
		}
	}
	return -1
}

func TestReplay_ACycleBouncingOnOneFingerprintStopsByTheThirdBackwardEdge(t *testing.T) {
	bounce := inputOf(convergence.LoopCycle, cycleRounds("audit->build", "audit->build", "retro->tdd", "audit->build"))
	bounce.Fixer, bounce.Headroom = deepJudge, v3bTables

	if r := firstStop(t, bounce); r != 2 {
		t.Fatalf("the cycle stopped after judgment %d, want 2: the third backward edge (retro->tdd, same fingerprint) is refused", r)
	}
	second := decide(t, prefix(bounce, 1))
	requireOutcome(t, second, convergence.ActionContinue, 1)
	if second.FixerRaise != topJudge {
		t.Fatalf("the second backward edge: fixer raise %+v, want the fixer's effort raised", second.FixerRaise)
	}

	wideBudget := bounce
	wideBudget.Config.MaxBackwardEdges = 32
	if r := firstStop(t, wideBudget); r != 2 {
		t.Fatalf("with a 32-edge budget the cycle stopped after judgment %d, want 2: the fingerprint, not the crash guard, ends it", r)
	}
}

func TestReplay_ACycleNeverCrawlsToTheCrashGuard(t *testing.T) {
	edges := make([]string, 33)
	for k := range edges {
		edges[k] = "audit->build"
	}
	in := inputOf(convergence.LoopCycle, cycleRounds(edges...))
	for k := range in.Rounds {
		in.Rounds[k].Fingerprint = ""
	}

	if r := firstStop(t, in); r != 3 {
		t.Fatalf("the cycle stopped after judgment %d, want 3: max_backward_edges 3 refuses a fourth edge", r)
	}
	for r := 3; r <= 32; r++ {
		if d := decide(t, prefix(in, r)); d.Action == convergence.ActionContinue {
			t.Fatalf("after %d backward edges: Continue, want an exit long before the 32-iteration crash guard", r)
		}
	}
}
