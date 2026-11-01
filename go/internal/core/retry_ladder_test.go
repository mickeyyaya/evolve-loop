package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const ladderFloorReason = "build deliverable failed contract: [cover_changed] 11 of 26 changed functions are below 100%"

type ladderProbe struct {
	*escalationProbe
	requests []PhaseRequest
}

func (p *ladderProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.requests = append(p.requests, req)
	return p.escalationProbe.Run(ctx, req)
}

func newLadderProbe(approveAfter int) *ladderProbe {
	return &ladderProbe{escalationProbe: &escalationProbe{
		phase: "build", threshold: neverDemotingThreshold, approveAfter: approveAfter, reasonPerBlock: []string{ladderFloorReason},
	}}
}

func ladderWorkflow(stage string) policy.WorkflowConfig {
	wf := policy.Policy{}.WorkflowConfig()
	wf.RetryLadder.Stage = stage
	return wf
}

func captureSignals() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func runLadderCycle(t *testing.T, stage string, probe *ladderProbe) []signalcenter.Event {
	t.Helper()
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	center, events := captureSignals()
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners, WithReviewer(probe),
		WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 1}), WithWorkflowConfig(ladderWorkflow(stage)), WithSignalCenter(center))
	_, _ = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	return *events
}

func TestContractCorrection_ShadowAsksTheDeciderAndDispatchesAsToday(t *testing.T) {
	t.Parallel()
	probe := newLadderProbe(0)
	events := runLadderCycle(t, policy.RetryLadderShadow, probe)

	if len(probe.requests) != 2 {
		t.Fatalf("shadow dispatched %d build tries, want 2 (the try and its one correction)", len(probe.requests))
	}
	for i, req := range probe.requests {
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "ladder") || req.Assignment.Tier != "" || req.Assignment.Effort != "" || req.Ladder.LadderTier() != "" {
			t.Fatalf("shadow try %d changed the dispatch: %s", i, raw)
		}
	}
	if !strings.HasPrefix(probe.requests[1].CorrectionDirective, "Your previous output for this phase was REJECTED") {
		t.Fatalf("the shadow correction lost its directive: %q", probe.requests[1].CorrectionDirective)
	}
	correction := probe.requests[1]
	correction.CorrectionDirective, correction.Attempt = probe.requests[0].CorrectionDirective, probe.requests[0].Attempt
	first, err := json.Marshal(probe.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(correction)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("the shadow correction differs from the try beyond its directive and attempt:\n%s\n%s", first, second)
	}
	decisions := eventsWithCode(events, CodeLadderDecision)
	if len(decisions) != 1 {
		t.Fatalf("shadow recorded %d LADDER_DECISION events, want 1: %+v", len(decisions), events)
	}
	d := decisions[0]
	if d.Fields["stage"] != policy.RetryLadderShadow || d.Fields["origin"] != originContractCorrection || d.Fields["class"] != string(retryladder.Capability) ||
		d.Fields["rung"] != string(retryladder.R0) || d.Fields["next"] == "" || d.Fields["next"] == string(retryladder.R0) || d.Module != signalcenter.ModuleLadder {
		t.Fatalf("shadow decision = %+v, want the raise that the ladder would choose", d)
	}
}

func TestContractCorrection_EnforceEscalatesToAFreshRaisedTryAfterOneCorrection(t *testing.T) {
	t.Parallel()
	probe := newLadderProbe(0)
	events := runLadderCycle(t, policy.RetryLadderEnforce, probe)

	if len(probe.requests) != 4 {
		t.Fatalf("enforce dispatched %d build tries, want 4 (try, one correction, R2, R3, then stop)", len(probe.requests))
	}
	if probe.requests[1].Ladder != nil {
		t.Fatalf("the one correction ran at a raised rung: %+v", probe.requests[1].Ladder)
	}
	raised := probe.requests[2]
	if raised.Ladder == nil || raised.Ladder.Rung != retryladder.R2 || raised.Ladder.Context != retryladder.ContextFresh ||
		raised.Ladder.Stage != retryladder.StageEnforce || raised.Ladder.LadderTier() != "deep" {
		t.Fatalf("the raised try = %+v, want a fresh R2 try at tier deep", raised.Ladder)
	}
	if !strings.Contains(raised.CorrectionDirective, "Retry digest") || !strings.Contains(raised.CorrectionDirective, ladderFloorReason) {
		t.Fatalf("the raised try lacks the digest with the verbatim check:\n%s", raised.CorrectionDirective)
	}
	if strings.Contains(raised.CorrectionDirective, "Your previous output for this phase was REJECTED") {
		t.Fatalf("the raised try re-prompts the old correction text:\n%s", raised.CorrectionDirective)
	}
	finalTry := probe.requests[3]
	if finalTry.Ladder == nil || finalTry.Ladder.Rung != retryladder.R3 || finalTry.Ladder.Context != retryladder.ContextFresh ||
		finalTry.Ladder.Stage != retryladder.StageEnforce {
		t.Fatalf("the last try = %+v, want a fresh R3 try", finalTry.Ladder)
	}
	if !strings.Contains(finalTry.CorrectionDirective, "Retry digest") || !strings.Contains(finalTry.CorrectionDirective, ladderFloorReason) {
		t.Fatalf("the R3 try lacks the digest with the verbatim check:\n%s", finalTry.CorrectionDirective)
	}
	if len(eventsWithCode(events, CodeLadderStopSpec)) != 1 {
		t.Fatalf("an R3 try that fails on the same check must stop as spec: %+v", events)
	}
}

func TestContractCorrection_EnforceKeepsTheContractCLIEscalationOff(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := newLadderProbe(0)
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithReviewer(probe),
		WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 3}), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	_, _ = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	for i, req := range probe.requests {
		if !req.Driver.IsZero() {
			t.Fatalf("try %d moved to cli %q: at enforce the ladder, not the CLI escalation, picks the next try", i, req.Driver)
		}
	}
}

func TestAskLadder_AJudgeSeatNeverClimbs(t *testing.T) {
	t.Parallel()
	center, events := captureSignals()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)), WithSignalCenter(center))
	cr := &cycleRun{o: o, cs: CycleState{CycleID: 3}, req: CycleRequest{ProjectRoot: t.TempDir()}}
	for _, seat := range []Phase{PhaseAudit, Phase("adversarial-review"), Phase("code-review"), Phase("spec-verify")} {
		req := PhaseRequest{CorrectionDirective: "keep"}
		got, raised := cr.askLadder(ladderAsk{phase: seat, origin: "test", facts: retryladder.Facts{ContractRejected: true, Reason: "x"}}, req)
		if raised || got.Ladder != nil || got.CorrectionDirective != "keep" {
			t.Fatalf("judge seat %s climbed: %+v", seat, got)
		}
	}
	if len(*events) != 0 || cr.cs.Ladder != nil {
		t.Fatalf("a judge seat recorded a decision: %+v, state %+v", *events, cr.cs.Ladder)
	}
}

func TestAskLadder_TheDigestStaysInsideThePolicyBudget(t *testing.T) {
	t.Parallel()
	wf := ladderWorkflow(policy.RetryLadderEnforce)
	wf.RetryLadder.DigestTokens = 50
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf))
	cr := &cycleRun{o: o, cs: CycleState{CycleID: 3}, req: CycleRequest{ProjectRoot: t.TempDir()}}
	huge := "[cover_changed] " + strings.Repeat("function below full coverage; ", 400)
	got, raised := cr.askLadder(ladderAsk{phase: PhaseBuild, origin: "test", facts: retryladder.Facts{ContractRejected: true, Reason: huge}, failures: []string{huge}}, PhaseRequest{})
	if !raised {
		t.Fatalf("a capability failure at R0 did not raise")
	}
	if n := len(got.CorrectionDirective); n > 50*retryladder.DigestBytesPerToken {
		t.Fatalf("digest is %d bytes, over the 50-token budget", n)
	}
}

func TestAskLadder_TheRaisedTryCarriesTheChangedFilesAndTheHistory(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	cr := &cycleRun{o: o, cs: CycleState{CycleID: 3}, req: CycleRequest{ProjectRoot: t.TempDir()}}
	ask := ladderAsk{phase: PhaseBuild, origin: "test", facts: retryladder.Facts{ContractRejected: true, Reason: "[a] first"}, failures: []string{"[a] first"}}
	first, _ := cr.askLadder(ask, PhaseRequest{})
	ask.facts.Reason, ask.failures = "[b] second", []string{"[b] second"}
	second, raised := cr.askLadder(ask, first)
	if !strings.Contains(first.CorrectionDirective, "[a] first") {
		t.Fatalf("first raise lacks its check:\n%s", first.CorrectionDirective)
	}
	if raised && !strings.Contains(second.CorrectionDirective, "codes:a") {
		t.Fatalf("a later raise lacks the fingerprint history:\n%s", second.CorrectionDirective)
	}
	if cr.cs.Ladder == nil || len(cr.cs.Ladder.Fingerprints) != 2 {
		t.Fatalf("the cycle state lacks the two fingerprints: %+v", cr.cs.Ladder)
	}
}

func TestAskLadder_AnR1TryIsFreshAndCarriesTheVerbatimCheck(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	cr := &cycleRun{o: o, cs: CycleState{CycleID: 4}, req: CycleRequest{ProjectRoot: t.TempDir()}}
	const check = "audit class superseded-predicate-contradiction: TestC4_001 still red"
	got, raised := cr.askLadder(ladderAsk{phase: PhaseBuild, origin: "test", facts: retryladder.Facts{Reason: check}, failures: []string{check}}, PhaseRequest{})
	if !raised || got.Ladder == nil || got.Ladder.Rung != retryladder.R1 || got.Ladder.Context != retryladder.ContextFresh {
		t.Fatalf("an unknown failure at R0 = %+v, want a fresh R1 try", got.Ladder)
	}
	if !strings.Contains(got.CorrectionDirective, check) {
		t.Fatalf("the R1 prompt lacks the verbatim check:\n%s", got.CorrectionDirective)
	}
}

func TestLadderCodes_AreRegisteredUnderTheLadderModule(t *testing.T) {
	t.Parallel()
	for _, code := range []signalcenter.Code{CodeLadderDecision, CodeLadderStopSpec, CodeLadderNoHeadroom, CodeLadderAuditFix} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleLadder {
			t.Fatalf("%s registered=%v module=%q, want the ladder module", code, ok, m)
		}
	}
}
