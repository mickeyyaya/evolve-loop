package inboxcurator

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestCurate_ALandingRunsTheMatchAndARoutingChangeRunsTheRerouteCheck(t *testing.T) {
	gate := func(p string) bool { return p == "go/internal/guards/ship.go" }
	onLane := inboxbatch.Item{ID: "gate", Kind: "feature", Route: inboxbatch.RouteLaneValue, Files: []string{"go/internal/guards/ship.go"}}
	in := Inputs{Open: []inboxbatch.Item{poolOpenItem(), onLane}, IsProtected: gate, MinShare: 0.5, Thresholds: Thresholds{Refuse: 0.5},
		HasTest: testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops")}

	in.Changes = []Change{{Commit: "c1", Paths: []string{"go/internal/fleet/pool.go"}}}
	if got := Curate(in); len(got) != 1 || got[0].ID != "rolling-pool" {
		t.Fatalf("a landing on the pool = %+v, want only the consume of rolling-pool", got)
	}
	in.Changes = []Change{{Commit: "c2", Paths: []string{"go/internal/guards/integrity_surface.go"}}}
	if got := Curate(in); len(got) != 1 || got[0].Kind != ProposeReroute || got[0].ID != "gate" {
		t.Fatalf("a manifest change = %+v, want only the reroute of gate", got)
	}
}

func TestCurate_TheBackstopRunsEveryCheckAndKeepsTheDeterministicProposalOfAnItem(t *testing.T) {
	dup := poolOpenItem()
	dup.ID, dup.Path, dup.CreatedAt = "rolling-pool-again", "z.json", "2027-01-01T00:00:00Z"
	in := Inputs{Open: []inboxbatch.Item{poolOpenItem(), dup}, MinShare: 0.5, Thresholds: Thresholds{Refuse: 0.5}, Backstop: true,
		HasTest: testsIn("TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce", "TestLease_ALanePastItsDeadlineStops"),
		Changes: []Change{{Commit: "c1", Paths: []string{"go/internal/fleet/pool.go"}, Tests: []string{"TestLease_ALanePastItsDeadlineStops"}, When: landedAt}}}

	got := Curate(in)

	kinds := map[string]bool{}
	for _, p := range got {
		kinds[string(p.Kind)+" "+p.ID] = true
		if p.Kind == ProposeConsume && p.ID == "rolling-pool" && (!p.Deterministic || p.Trigger != TriggerLanding) {
			t.Errorf("the consume of rolling-pool = %+v, want the deterministic landing proposal, not the sweep one", p)
		}
	}
	if !kinds["consume rolling-pool"] || !kinds["consume rolling-pool-again"] || !kinds["merge rolling-pool-again"] || len(got) != 3 {
		t.Errorf("Curate = %+v, want consume of both and one merge", got)
	}
}

func TestProposal_EventIsAnInboxSignalThatNamesTheStageAndTheApply(t *testing.T) {
	p := Proposal{Kind: ProposeConsume, ID: "a", Trigger: TriggerLanding, Deterministic: true, Evidence: []string{"commit c", "tests found: T"}}

	e := p.Event("enforce", true)

	if e.Code != CodeProposal || e.Module != signalcenter.ModuleInbox || e.Fields["applied"] != "true" || e.Fields["stage"] != "enforce" || e.Fields["evidence"] != "commit c; tests found: T" {
		t.Errorf("Event = %+v, want the proposal code with stage, applied and evidence", e)
	}
}

func TestFirstPerItem_KeepsTheFirstUnlessALaterOneIsDeterministic(t *testing.T) {
	judged := Proposal{Kind: ProposeConsume, ID: "a", Trigger: TriggerSweep}
	verified := Proposal{Kind: ProposeConsume, ID: "a", Trigger: TriggerLanding, Deterministic: true}

	got := firstPerItem([]Proposal{judged, verified, judged, {Kind: ProposeMerge, ID: "a"}})

	if len(got) != 2 || !got[0].Deterministic || got[0].Trigger != TriggerLanding || got[1].Kind != ProposeMerge {
		t.Errorf("firstPerItem = %+v, want the deterministic consume, then the merge", got)
	}
}

func TestProposalKind_SpellsEachKindAsItsSignalField(t *testing.T) {
	for kind, want := range map[ProposalKind]string{ProposeConsume: "consume", ProposeReroute: "reroute", ProposeMerge: "merge"} {
		if got := (Proposal{Kind: kind}).Event("shadow", false).Fields["kind"]; got != want {
			t.Errorf("kind field = %q, want %q", got, want)
		}
	}
}
