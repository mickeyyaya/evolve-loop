package advisor

// mint_test.go — the recursion guard and the mint drops reported at decision
// time (ADR-0103 unit 04 §6 tests 25, 26; the core mint tests moved verbatim
// in intent through Plan).

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The WS1-S2 recursion guard (ADR-0052 D1, primary defense): a mint whose
// name is a reserved control-plane identity is dropped with an observable
// reason; legitimate mints pass through untouched.
func TestMintConfigsFrom_RejectsAdvisorRoleMint(t *testing.T) {
	entries := []router.PhasePlanEntry{
		{Phase: "router", Run: true, Mint: &router.MintSpec{Prompt: "be a router"}},
		{Phase: "evolve-router", Run: true, Mint: &router.MintSpec{Prompt: "x"}},
		{Phase: "Failure-Advisor", Run: true, Mint: &router.MintSpec{Prompt: "x"}}, // case-insensitive
		{Phase: "new-helper", Run: true, Mint: &router.MintSpec{Prompt: "legit"}},
		{Phase: "plain", Run: true},
	}
	got, rejected := MintConfigsFrom(entries)
	if len(got) != 1 || got[0].Name != "new-helper" {
		t.Fatalf("recursion guard failed: minted configs = %+v, want only new-helper", got)
	}
	if len(rejected) != 3 || rejected[0].Phase != "router" || rejected[1].Phase != "evolve-router" || rejected[2].Phase != "Failure-Advisor" {
		t.Fatalf("the drops are returned in order: %+v", rejected)
	}
	for _, name := range []string{"router", "EVOLVE-ROUTER", " advisor ", "phase-advisor", "failure-advisor", "Evolve-Failure-Advisor"} {
		if ReservedMintReason(name) == "" {
			t.Errorf("ReservedMintReason(%q) must be non-empty (case-insensitive, trimmed)", name)
		}
	}
	if ReservedMintReason("new-helper") != "" {
		t.Error("ReservedMintReason(new-helper) must be empty (allowed)")
	}
	if mints, rej := MintConfigsFrom(nil); mints != nil || rej != nil {
		t.Error("no entries mint nothing")
	}
}

// Test 25 — one ADVISOR_MINT_REJECTED per drop, emitted by the entry point
// with the decision stamp; the plan still returns with the legal mint.
func TestPlan_EmitsMintRejectedOncePerDropWithTheDecisionStamp(t *testing.T) {
	stdout := `[{"phase":"scout","run":true},{"phase":"router","run":true,"mint":{"prompt":"be a router"}},{"phase":"Advisor","run":true,"mint":{"prompt":"x"}},{"phase":"new-helper","run":true,"mint":{"prompt":"legit"}}]`
	for _, c := range []struct {
		name, decision, contract, origin string
		launch                           func(*Advisor, router.RouteInput) (*router.PhasePlan, error)
	}{
		{"plan", "plan", "router", "Advisor.Plan", (*Advisor).Plan},
		{"replan", "replan", "router-replan", "Advisor.RePlan", (*Advisor).RePlan},
	} {
		in := tempInput(t)
		in.Current = "scout"
		in.Cycle = 1642
		a, got := observed(t, &fakeLauncher{stdout: stdout}, defaultIdentity())
		plan, err := c.launch(a, in)
		if err != nil || len(plan.MintPhases) != 1 || plan.MintPhases[0].Name != "new-helper" || len(plan.Entries) != 4 {
			t.Fatalf("%s: the plan stands with the legal mint: %+v %v", c.name, plan, err)
		}
		if len(*got) != 2 {
			t.Fatalf("%s: one event per drop: %+v", c.name, *got)
		}
		for i, want := range []string{"router", "Advisor"} {
			e := (*got)[i]
			if e.Code != CodeMintRejected || e.Module != signalcenter.ModuleAdvisor || e.Kind != signalcenter.KindAdvisorWarning || e.Severity != signalcenter.SeverityWarn {
				t.Errorf("%s: event %d shape: %+v", c.name, i, e)
			}
			if e.Origin != c.origin || e.Cycle != 1642 || e.Phase != "scout" || e.Fields["decision"] != c.decision || e.Fields["contract"] != c.contract || e.Fields["step"] != "mint" {
				t.Errorf("%s: event %d stamp: %+v", c.name, i, e)
			}
			if e.Fields["minted_phase"] != want || e.Reason != ReservedMintReason(want) {
				t.Errorf("%s: event %d names the dropped mint and the guard's reason: %+v", c.name, i, e)
			}
		}
	}
}

// Test 26 — an unparseable response is one ADVISOR_RESPONSE_UNPARSEABLE with
// its cause and the wrapped error the orchestrator prints.
func TestParse_UnparseableResponseWarnsOnceAndReturnsTheWrappedError(t *testing.T) {
	for _, c := range []struct {
		name, stdout, want, cause, decision, contract, artifact string
		launch                                                  func(*Advisor, router.RouteInput) error
	}{
		{"proposal prose", "I could not decide.", "routing proposer: no JSON object in proposer output", "no_json", "proposal", "router-proposal", "routing-proposal.json", func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
		{"proposal empty", `{"justification":"nothing"}`, "routing proposer: empty proposal", "empty", "proposal", "router-proposal", "routing-proposal.json", func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
		{"plan malformed", `[{"phase":}]`, "phase advisor: parse phase plan: invalid character '}' looking for beginning of value", "invalid_json", "plan", "router", "routing-plan.json", func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"plan empty", "[]", "phase advisor: empty phase plan", "empty", "plan", "router", "routing-plan.json", func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan prose", "no array here", "phase advisor: no JSON array in plan output", "no_json", "replan", "router-replan", "routing-replan.json", func(a *Advisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
	} {
		a, got := observed(t, &fakeLauncher{stdout: c.stdout}, defaultIdentity())
		err := c.launch(a, tempInput(t))
		if err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
			continue
		}
		e := assertOneEvent(t, *got, CodeResponseUnparseable, map[string]string{
			"step": "parse", "cause": c.cause, "decision": c.decision, "contract": c.contract, "artifact": c.artifact,
		})
		if e.Reason != strings.TrimPrefix(err.Error(), decisionRows[decisionForKind(c.decision)].errPfx+": ") || e.Fields["stdout_bytes"] == "" {
			t.Errorf("%s: reason is the unwrapped parse error, stdout_bytes present: %+v", c.name, e)
		}
	}
}

func decisionForKind(kind string) decision {
	for i, r := range decisionRows {
		if r.kind == kind {
			return decision(i)
		}
	}
	return decisionPlan
}

// The common path is untouched: a plan with no mint sub-objects yields zero
// MintPhases and no event (moved from core).
func TestPlan_NoMint_EmptyMintPhases(t *testing.T) {
	a, got := observed(t, &fakeLauncher{stdout: `[{"phase":"scout","run":true},{"phase":"triage","run":false}]`}, defaultIdentity())
	plan, err := a.Plan(tempInput(t))
	if err != nil || len(plan.MintPhases) != 0 || len(*got) != 0 {
		t.Errorf("no mint ⇒ no MintPhases, no event: %+v %v %+v", plan, err, *got)
	}
}
