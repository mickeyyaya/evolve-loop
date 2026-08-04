package core

// contract_escalation_test.go — RED-first coverage for CONTRACT-BLOCK CLI
// ESCALATION (inbox contract-block-cli-escalation, P1 weight 0.95; twice
// confirmed live: batch-19 cycles 1171/1172 adversarial-review and batch-21
// cycle-1215 triage, both on agy-tmux).
//
// The live defect: reviewAndGuard's correction ladder re-dispatches the SAME
// profile CLI after a contract block. cli_fallback fires only on infra exit
// codes {80,81,85,124,127}, never on a contract violation, so a CLI that
// systematically mis-formats a deliverable burns every correction and the
// contract-gate breaker demotes enforce→advisory — a gate-WEAKENING outcome.
//
// These tests drive the REAL Orchestrator.profileForModelRouting seam (real
// .evolve/profiles/*.json on disk) through RunCycle, so they exercise the
// production resolution path rather than a stub.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

// writeCLIProfile writes .evolve/profiles/<agent>.json under root carrying the
// primary cli + optional cli_fallback chain. Deliberately minimal (no
// $include_policy sentinels) so profiles.Loader.Get parses it without a
// tool-policy file — same shape writeBuilderProfile uses.
func writeCLIProfile(t *testing.T, root, agent, cli string, fallback []string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	doc := map[string]any{"name": agent, "role": agent, "cli": cli, "model_tier_default": "balanced"}
	if len(fallback) > 0 {
		doc["cli_fallback"] = fallback
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), body, 0o644); err != nil {
		t.Fatalf("write %s.json: %v", agent, err)
	}
}

// escalationProbe is BOTH the phase runner and the deliverable reviewer for one
// phase, coupled through the single fact the live defect turns on: WHICH CLI
// produced the deliverable now under review. It reproduces the production
// contract gate's breaker semantics (deliverable.Reviewer): each non-compliant
// deliverable is a BLOCK, `threshold` consecutive blocks demote enforce→advisory
// (Approve + Demoted), and a compliant deliverable resets the counter.
//
// compliantCLI == "" models batch-19/21: no re-dispatch ever complies.
type escalationProbe struct {
	phase        string
	compliantCLI string
	threshold    int
	// approveAfter (>0) approves once this many reviews have happened, so a test
	// can stop the ladder after a chosen number of blocks.
	approveAfter int
	// reasonPerBlock, when non-empty, supplies the violation text per consecutive
	// block (block n uses index n-1, the last entry repeating for later blocks).
	// Empty ⇒ every block reports the same reason, which is what the pre-cycle-1289
	// tests assume. This is the ONE axis the fingerprint gate turns on: whether
	// block 2's violation is the SAME defect as block 1's.
	reasonPerBlock []string

	lastCLI    string   // ModelRoutingCLI of the most recent dispatch
	dispatched []string // ModelRoutingCLI per dispatch, in order
	// blocks is the consecutive-block count the gate reports as
	// ReviewResult.Blocks. Pre-seeding it models a breaker left HOT by an earlier
	// cycle or phase — the production counter is a single global file, not a
	// per-phase, per-cycle counter.
	blocks  int
	reviews int
}

func (p *escalationProbe) Name() string { return p.phase }

func (p *escalationProbe) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.lastCLI = req.ModelRoutingCLI
	p.dispatched = append(p.dispatched, req.ModelRoutingCLI)
	return PhaseResponse{Phase: p.phase, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (p *escalationProbe) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != p.phase {
		return ReviewResult{Approve: true}
	}
	p.reviews++
	if p.approveAfter > 0 && p.reviews > p.approveAfter {
		p.blocks = 0
		return ReviewResult{Approve: true}
	}
	if p.compliantCLI != "" && p.lastCLI == p.compliantCLI {
		p.blocks = 0
		return ReviewResult{Approve: true}
	}
	p.blocks++
	reason := p.phase + " deliverable failed contract: [missing_section] required section 'Findings' not found"
	if n := len(p.reasonPerBlock); n > 0 {
		idx := p.blocks - 1
		if idx >= n {
			idx = n - 1
		}
		reason = p.reasonPerBlock[idx]
	}
	if p.blocks >= p.threshold {
		return ReviewResult{Approve: true, Demoted: true, Reason: reason, Blocks: p.blocks}
	}
	return ReviewResult{Approve: false, Reason: reason, Blocks: p.blocks}
}

// runEscalationCycle drives a full RunCycle with probe wired as BOTH the build
// runner and the reviewer, returning the ledger for demotion-evidence assertions.
func runEscalationCycle(t *testing.T, root string, probe *escalationProbe) (*fakeLedger, error) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, led, runners, WithReviewer(probe))
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	return led, err
}

// neverDemotingThreshold keeps the probe's breaker shut for the whole ladder so
// a test observes the correction re-dispatches themselves, not the demotion.
const neverDemotingThreshold = 99

// TestContractCorrection_SecondBlockEscalatesToProfileFallback is the primary
// acceptance criterion (a): after the SECOND consecutive contract block the
// re-dispatch must go to the profile's cli_fallback, not the same
// contract-violating CLI. The profile's PRIMARY routing must be untouched —
// dispatch 1 (initial) and dispatch 2 (correction 1) carry no routing override.
func TestContractCorrection_SecondBlockEscalatesToProfileFallback(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[0] != "" {
		t.Errorf("initial dispatch carried ModelRoutingCLI=%q, want \"\" — the phase's PRIMARY routing must be untouched (scoping constraint: escalate only the re-dispatch that already failed the contract)", probe.dispatched[0])
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — the SECOND consecutive contract block must re-dispatch on the profile's cli_fallback BEFORE the third strike opens the circuit", probe.dispatched[2])
	}
}

// TestContractCorrection_FirstBlockDoesNotEscalate is acceptance criterion (c):
// one bad turn is not a CLI verdict. Correction 1 must re-dispatch on the SAME
// (primary) routing — escalating on the first block would reroute 99% of a
// phase's honest single-turn slips onto a different CLI.
func TestContractCorrection_FirstBlockDoesNotEscalate(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	// Block once, then approve: the ladder stops after correction 1.
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold, approveAfter: 1}

	if _, err := runEscalationCycle(t, root, probe); err != nil {
		t.Fatalf("RunCycle should proceed after one correction: %v", err)
	}
	if len(probe.dispatched) != 2 {
		t.Fatalf("build dispatched %d times (%v), want 2 (initial + 1 correction)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[1] != "" {
		t.Errorf("correction 1 dispatched on ModelRoutingCLI=%q, want \"\" \u2014 the FIRST contract block must NOT escalate the CLI (one bad turn is not a CLI verdict)", probe.dispatched[1])
	}
}

// TestContractCorrection_HotBreakerEscalatesOnFirstCorrection: the escalation
// keys off the GATE's consecutive-block count, not a locally re-counted
// correction ordinal. The production breaker is one global file
// (.evolve/contract-gate-breaker.json) reset only by a clean verify, so a cycle
// that aborted mid-ladder leaves it hot. Arriving already at block 2 on the FIRST
// block of this phase, escalation must still get its shot before the third
// strike \u2014 with a correction-ordinal trigger it never would.
func TestContractCorrection_HotBreakerEscalatesOnFirstCorrection(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: 3, blocks: 1} // breaker left hot

	led, err := runEscalationCycle(t, root, probe)
	if err != nil {
		t.Fatalf("the third block demotes and therefore approves: %v", err)
	}
	if len(probe.dispatched) < 2 {
		t.Fatalf("build dispatched %v, want at least initial + 1 correction", probe.dispatched)
	}
	if probe.dispatched[1] != "codex-tmux" {
		t.Errorf("correction 1 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" \u2014 with the breaker already at 1, this phase's first block IS the second consecutive block", probe.dispatched[1])
	}
	var sawDemotion bool
	for _, e := range led.entries {
		if e.Kind == ledgerKindContractGateDemoted {
			sawDemotion = true
			if !strings.Contains(e.Action, "escalated=true") {
				t.Errorf("demotion ledger Action=%q, want it to record that escalation DID run", e.Action)
			}
		}
	}
	if !sawDemotion {
		t.Error("expected the circuit to open after the escalated CLI also failed")
	}
}

// TestContractCorrection_NonContractRejectionNeverEscalates is the scoping guard
// for the OTHER gates chained at the same seam (evalgate / topngate / triagecap /
// the build floor): they report Blocks==0 because they keep no contract-block
// counter. A wrong-task binding or an over-capacity triage plan is not a
// format-compliance failure, so a different CLI is not the remedy and the ladder
// must behave exactly as before.
func TestContractCorrection_NonContractRejectionNeverEscalates(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}
	// A topngate-shaped rejection: real reason, no block counter.
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "build report task slug outside triage top_n"}},
	}
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(rev))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	}); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	for i, cli := range probe.dispatched {
		if cli != "" {
			t.Errorf("dispatch %d carried ModelRoutingCLI=%q, want \"\" \u2014 a non-contract rejection must never escalate the CLI (dispatched=%v)", i, cli, probe.dispatched)
		}
	}
}

// TestContractCorrection_NoDeclaredFallbackEscalatesToUniversalClaude is
// acceptance criterion (b): a profile that declares NO cli_fallback still
// escalates — to the universal claude fallback — so the escape hatch is CLI
// escalation rather than gate demotion for every profile, not just the ones
// whose operator happened to configure a chain.
func TestContractCorrection_NoDeclaredFallbackEscalatesToUniversalClaude(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != universalContractFallbackCLI {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want %q — a profile with no declared cli_fallback must escalate to the universal claude fallback", probe.dispatched[2], universalContractFallbackCLI)
	}
}

// TestContractCorrection_SameFamilyFallbackIsNotAnEscalation is the
// precision/anti-no-op guard: escalation must change the CLI FAMILY. A profile
// already on the claude family with no other family in its chain has no
// escalation target, so the re-dispatch stays on the primary rather than
// "escalating" to the CLI that just failed the contract.
func TestContractCorrection_SameFamilyFallbackIsNotAnEscalation(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", universalContractFallbackCLI, nil)
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"\" — a primary already on the universal-fallback family has NO escalation target; re-pointing at the same family is not an escalation", probe.dispatched[2])
	}
}

// TestContractCorrection_CompliantFallbackPreventsCircuitOpen is acceptance
// criterion (d) — the whole point of the fix. The primary CLI never satisfies
// the contract but the escalation target does, so the third strike never lands:
// the cycle completes, the contract gate is NOT demoted, and no demotion
// evidence is recorded.
func TestContractCorrection_CompliantFallbackPreventsCircuitOpen(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", compliantCLI: "codex-tmux", threshold: 3}

	led, err := runEscalationCycle(t, root, probe)
	if err != nil {
		t.Fatalf("RunCycle must complete once the escalated CLI satisfies the contract: %v", err)
	}
	if len(probe.dispatched) != 3 || probe.dispatched[2] != "codex-tmux" {
		t.Fatalf("dispatch chain = %v, want the 3rd dispatch on codex-tmux", probe.dispatched)
	}
	if probe.blocks >= probe.threshold {
		t.Errorf("consecutive blocks = %d, want < %d — a compliant escalation must reset the breaker before it opens", probe.blocks, probe.threshold)
	}
	for _, e := range led.entries {
		if e.Kind == ledgerKindContractGateDemoted {
			t.Errorf("recorded a %q ledger entry, but the circuit never opened: %+v", ledgerKindContractGateDemoted, e)
		}
	}
	if _, statErr := os.Stat(dispositionrouter.PendingActionsPath(filepath.Join(root, ".evolve", "escalations"))); statErr == nil {
		t.Error("staged an escalation intent although the circuit never opened")
	}
}

// TestContractCorrection_CircuitOpenWarnsAndFilesItem is acceptance criterion
// (e): when even the escalated CLI fails the contract, the circuit still opens
// as the LAST resort — but the demotion is no longer a single invisible log
// line. It records a cycle-visible ledger entry AND stages an autofile intent
// naming the demoted phase + CLI (staged, never written straight to
// .evolve/inbox — a mid-flight inbox write races inboxmover.Claim's os.Rename;
// recurrence.ApplyBoundary is the only sanctioned inbox writer).
func TestContractCorrection_CircuitOpenWarnsAndFilesItem(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: 3}

	led, err := runEscalationCycle(t, root, probe)
	if err != nil {
		t.Fatalf("a demoted gate approves, so the cycle must complete: %v", err)
	}
	if len(probe.dispatched) != 3 || probe.dispatched[2] != "codex-tmux" {
		t.Fatalf("dispatch chain = %v, want the 3rd dispatch escalated to codex-tmux", probe.dispatched)
	}
	var demoted *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == ledgerKindContractGateDemoted {
			demoted = &led.entries[i]
		}
	}
	if demoted == nil {
		kinds := make([]string, 0, len(led.entries))
		for _, e := range led.entries {
			kinds = append(kinds, e.Kind)
		}
		t.Fatalf("no %q ledger entry — a gate demotion must be visible in the cycle record, not just one stderr line (kinds=%v)", ledgerKindContractGateDemoted, kinds)
	}
	if demoted.Role != "build" {
		t.Errorf("demotion ledger entry Role=%q, want \"build\" (the demoted phase)", demoted.Role)
	}

	path := dispositionrouter.PendingActionsPath(filepath.Join(root, ".evolve", "escalations"))
	intents, lerr := dispositionrouter.LoadIntents(path)
	if lerr != nil {
		t.Fatalf("LoadIntents(%s): %v", path, lerr)
	}
	if len(intents) != 1 {
		t.Fatalf("staged %d intents, want exactly 1 autofile intent for the demotion", len(intents))
	}
	in := intents[0]
	if in.Action != dispositionrouter.ActionAutofile {
		t.Errorf("staged intent Action=%q, want %q", in.Action, dispositionrouter.ActionAutofile)
	}
	if in.Route != dispositionrouter.RouteConsole {
		t.Errorf("staged intent Route=%q, want %q — a demoted gate is pipeline machinery, operator-owned", in.Route, dispositionrouter.RouteConsole)
	}
	if in.Weight <= 0 {
		t.Errorf("staged intent Weight=%v, want > 0 (a zero weight files a bottom-ranked item the queue never reaches)", in.Weight)
	}
	blob := in.ItemID + "|" + in.Pattern + "|" + in.Reason
	if !strings.Contains(blob, "build") {
		t.Errorf("staged intent does not name the demoted phase: %+v", in)
	}
	if !strings.Contains(blob, "codex-tmux") {
		t.Errorf("staged intent does not name the CLI the gate was demoted on: %+v", in)
	}
}

// TestFormatContractGateDemotionWarn pins the operator-facing WARN's content:
// the demoted phase and the CLI must both be named (the batch-19 log line named
// neither, which is why two batches lost cycles to an invisible demotion).
func TestFormatContractGateDemotionWarn(t *testing.T) {
	t.Parallel()
	reason := "triage deliverable failed contract: [missing_failure_block] schema_version 2 required"
	got := formatContractGateDemotionWarn("triage", "agy-tmux", true, reason)
	for _, want := range []string{"WARN", "CONTRACT GATE DEMOTED", "triage", "agy-tmux", "advisory", "missing_failure_block"} {
		if !strings.Contains(got, want) {
			t.Errorf("demotion WARN missing %q: %s", want, got)
		}
	}
	if !strings.Contains(got, "escalation already ran") {
		t.Errorf("escalated=true must SAY escalation ran: %s", got)
	}
	// The unconditional "escalation was attempted" claim is exactly the kind of
	// line an operator trusts and acts on: it must be false only when it is false.
	notTried := formatContractGateDemotionWarn("triage", "agy-tmux", false, reason)
	if !strings.Contains(notTried, "did NOT run") {
		t.Errorf("escalated=false must NOT claim escalation was attempted: %s", notTried)
	}
}

// TestChainReviewers_PropagatesDemotedThroughApproval is the WIRING proof for
// the demotion signal: production mounts the contract gate INSIDE
// core.ChainReviewers (cmd_cycle.go), and the chain used to rebuild a bare
// ReviewResult{Approve:true} on the all-approve path — which would silently
// swallow the contract gate's Demoted flag and leave the orchestrator blind.
func TestChainReviewers_PropagatesDemotedThroughApproval(t *testing.T) {
	t.Parallel()
	chain := ChainReviewers(
		noopReviewer{},
		stubReviewer{result: ReviewResult{Approve: true, Demoted: true, Reason: "circuit open"}},
		noopReviewer{},
	)
	got := chain.Review(context.Background(), ReviewInput{Phase: "audit"})
	if !got.Approve {
		t.Fatalf("chain must approve when every reviewer approves; got %+v", got)
	}
	if !got.Demoted {
		t.Error("ChainReviewers dropped Demoted on the approve path — the orchestrator would never see the contract gate's demotion")
	}
	if got.Reason != "circuit open" {
		t.Errorf("Reason=%q, want the demoting reviewer's reason carried through", got.Reason)
	}
}

// TestChainReviewers_CarriesDemotionThroughLaterRejection is the OTHER half of
// the chain wiring, and the one that actually bites triage. Production order is
// buildFloor → evalgate → contract → triagecap → topngate (cmd_cycle.go), so the
// contract gate can demote and a LATER gate still reject the same deliverable. If
// the chain returned that rejection verbatim, the demotion evidence would be
// dropped, the gate would be permanently advisory, and — because the breaker stays
// over threshold — every subsequent review would drop it again: the exact
// invisibility this fix exists to remove.
func TestChainReviewers_CarriesDemotionThroughLaterRejection(t *testing.T) {
	t.Parallel()
	chain := ChainReviewers(
		stubReviewer{result: ReviewResult{Approve: true, Demoted: true, Reason: "contract circuit open", Blocks: 3}},
		stubReviewer{result: ReviewResult{Approve: false, Reason: "triage coverage floor above capacity"}},
	)
	got := chain.Review(context.Background(), ReviewInput{Phase: "triage"})
	if got.Approve {
		t.Fatal("a later gate's rejection must still short-circuit the chain")
	}
	if !got.Demoted {
		t.Error("the contract gate's demotion was dropped because a LATER gate rejected — the demotion still happened and must be reported")
	}
	if got.Blocks != 3 {
		t.Errorf("Blocks=%d, want 3 carried from the contract gate (the escalation trigger reads it)", got.Blocks)
	}
	if got.Reason != "triage coverage floor above capacity" {
		t.Errorf("Reason=%q — the deciding reviewer's own reason must stay verbatim", got.Reason)
	}
}

// TestUniversalContractFallbackMatchesLLMRouteDefault is the drift guard: the
// escalation's universal fallback must be exactly the CLI llmroute resolves
// when no env, no profile and no pin name one. Two literals that must agree
// cannot be allowed to drift silently.
func TestUniversalContractFallbackMatchesLLMRouteDefault(t *testing.T) {
	t.Parallel()
	plan := llmroute.Resolve("builder", "build", "balanced", nil, nil, nil, nil)
	if len(plan.Candidates) == 0 {
		t.Fatal("llmroute.Resolve returned no candidates")
	}
	if plan.Candidates[0] != universalContractFallbackCLI {
		t.Errorf("universalContractFallbackCLI=%q but llmroute's no-profile default is %q — the escalation target must be the same universal fallback the dispatch resolver uses", universalContractFallbackCLI, plan.Candidates[0])
	}
}

// ============================================================================
// cycle-1289 — FINGERPRINT-GATED ESCALATION TRIGGER
//
// The gap the landed PR #390 mechanism leaves open: the trigger at
// cyclerun_review.go counts blocks (`rr.Blocks >= contractEscalateAtBlock`) and
// never asks whether block 2 is the SAME defect as block 1. Two genuinely
// different contract violations on one phase (block 1 misses a section heading,
// block 2 misses the verdict sentinel) then read as one incapable-CLI signature
// and spend round 2's budget on a different family for no reason. The inbox item
// (.evolve/inbox/2026-08-04T07-15-00Z-contract-block-cli-escalation.json) states
// the fix: "integrate with the fingerprint breaker so identical blocks share
// identity" — i.e. reuse failure_digest.go's normalizeReasonForFingerprint, the
// blocker breaker's OWN identity primitive, rather than invent a second one.
//
// Three axes, encoded below and by the pre-existing tests:
//
//	NEGATIVE  differing violations       → NO escalation  (TestContractCorrection_DifferingBlockReasonsDoNotEscalate)
//	POSITIVE  same defect, noisy text    → escalates      (TestContractCorrection_NormalizedIdenticalReasonsEscalate)
//	EDGE      no prior reason observed   → escalates      (TestContractCorrection_HotBreakerEscalatesOnFirstCorrection, above)
//
// The EDGE axis is load-bearing and is why the gate is "prior reason known AND
// differing ⇒ suppress", not "equal ⇒ escalate": a breaker left HOT by an
// earlier cycle arrives at Blocks>=2 on this ladder's FIRST block, so there is
// no prior reason to compare. Requiring equality there would silently delete the
// hot-breaker escape hatch that PR #390's review established.
// ============================================================================

// TestContractCorrection_DifferingBlockReasonsDoNotEscalate is the NEGATIVE
// acceptance criterion: block 2 carrying a DIFFERENT violation than block 1 is
// two honest defects, not one incapable CLI, so correction 2 must re-dispatch on
// the phase's own routing exactly as it did before the escalation feature landed.
func TestContractCorrection_DifferingBlockReasonsDoNotEscalate(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_section] required section 'Findings' not found",
			"build deliverable failed contract: [missing_verdict] no VERDICT sentinel line found",
		},
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"\" — block 2's violation (%q) DIFFERS from block 1's (%q), so the two blocks are not one incapable-CLI signature and escalation must not fire",
			probe.dispatched[2], probe.reasonPerBlock[1], probe.reasonPerBlock[0])
	}
}

// TestContractCorrection_NormalizedIdenticalReasonsEscalate is the POSITIVE
// acceptance criterion AND the proof that the gate reuses the blocker breaker's
// identity primitive instead of comparing raw strings. The two reasons name the
// same defect and differ only in a go-test duration token — exactly the
// identity-noise class normalizeReasonForFingerprint (failure_digest.go) folds to
// "<dur>". A raw `block1.Reason == block2.Reason` comparison would suppress this
// escalation; the fingerprint-normalized one must still escalate.
func TestContractCorrection_NormalizedIdenticalReasonsEscalate(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_section] required section 'Findings' not found (1.478s)",
			"build deliverable failed contract: [missing_section] required section 'Findings' not found (1.495s)",
		},
	}
	// Guard the fixture itself: if these two reasons ever stop being
	// normalization-equal, this test would pass for the wrong reason.
	if a, b := normalizeReasonForFingerprint(probe.reasonPerBlock[0]), normalizeReasonForFingerprint(probe.reasonPerBlock[1]); a != b {
		t.Fatalf("fixture invalid: the two reasons must normalize identically, got %q vs %q", a, b)
	}
	if probe.reasonPerBlock[0] == probe.reasonPerBlock[1] {
		t.Fatal("fixture invalid: the two reasons must differ VERBATIM, else raw string equality would also pass")
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — the two blocks are the SAME defect under normalizeReasonForFingerprint (they differ only in a duration token), so the second consecutive block must still escalate",
			probe.dispatched[2])
	}
}

// ============================================================================
// CYCLE-1291 — VIOLATION-CODE-SET IDENTITY (the cycle-1289 audit defect)
//
// cycle-1289 shipped contractBlocksShareIdentity as a WHOLE-STRING compare of
// normalizeReasonForFingerprint(reason). The audit rejected it HIGH:
//
//	"contractBlocksShareIdentity compares whole summarize() strings, so a
//	 partially-repaired violation set (subset) reads as a different defect and
//	 suppresses [escalation]"
//	(.evolve/runs/cycle-1289/audit-fail-reason.json)
//
// The reason under comparison is deliverable.summarize() — a "; "-joined
// rendering of EVERY violation on that block ("[code] message"). So when block 1
// reports {missing_section, missing_verdict} and the correction closes ONE of
// them, block 2 reports {missing_verdict} alone. That is the SAME defect getting
// partially repaired — the strongest possible incapable-CLI signature, since the
// CLI demonstrably cannot close the remaining violation — yet the two rendered
// strings differ verbatim, normalizeReasonForFingerprint (which masks only
// durations and narrative verdicts, never violation-set MEMBERSHIP) leaves them
// differing, and the escalation is suppressed exactly when it is most warranted.
//
// The fix is to compare violation-CODE SETS, not rendered text. deliverable.
// Violation.Code is the stable identity primitive (go/internal/deliverable/
// deliverable.go:33-36) and is untouched by prose rewording or violation order.
// Same defect ⇔ the two blocks' code sets INTERSECT.
//
// IMPORT-CYCLE CONSTRAINT (cycle-644 reachability obligation — compiler-proven
// this cycle, do not re-litigate): internal/deliverable imports internal/core
// (reviewer.go:12, verifier.go:18) and core imports deliverable NOWHERE. So
// core.ReviewResult can NOT carry []deliverable.Violation — that is an import
// cycle and the criterion would be permanently unsatisfiable. The code set must
// reach core as plain data (codes parsed out of the rendered Reason, or a
// []string field on ReviewResult). These tests pin BEHAVIOUR through the real
// RunCycle ladder and deliberately do NOT pin either shape.
//
// Four axes, all driven through the production caller (RunCycle → reviewAndGuard):
//
//	POSITIVE  subset repair    {A,B} → {B}     → escalates  (001, THE audit defect)
//	POSITIVE  superset regress {B}   → {A,B}   → escalates  (002)
//	NEGATIVE  disjoint sets    {A,B} → {C,D}   → NO escalate (003)
//	EDGE      reordered/reworded same set      → escalates  (004)
//	EDGE      no [code] token at all           → escalates  (005, fail-safe)
// ============================================================================

// TestContractCorrection_SubsetRepairStillEscalates is THE cycle-1289 audit
// defect, encoded. Block 1 carries two violations; the correction closes the
// first, so block 2 carries only the second — a strict SUBSET of block 1's code
// set. Under whole-string identity the two summaries differ verbatim and the
// escalation is suppressed. Under code-set identity the sets intersect on
// missing_verdict, the blocks are one partially-repaired defect, and the second
// consecutive block must still escalate off the failing CLI family.
func TestContractCorrection_SubsetRepairStillEscalates(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_section] required section 'Findings' not found; [missing_verdict] no VERDICT sentinel line found",
			"build deliverable failed contract: [missing_verdict] no VERDICT sentinel line found",
		},
	}
	// Guard the fixture: the defect only exists if the two rendered reasons
	// differ verbatim AND stay differing after fingerprint normalization —
	// otherwise the pre-fix whole-string compare would already have escalated
	// and this test would pass for the wrong reason.
	if probe.reasonPerBlock[0] == probe.reasonPerBlock[1] {
		t.Fatal("fixture invalid: the two reasons must differ VERBATIM")
	}
	if a, b := normalizeReasonForFingerprint(probe.reasonPerBlock[0]), normalizeReasonForFingerprint(probe.reasonPerBlock[1]); a == b {
		t.Fatalf("fixture invalid: the two reasons must stay DIFFERENT under normalizeReasonForFingerprint (else the pre-fix compare already escalates), got %q == %q", a, b)
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — block 2's violation codes {missing_verdict} are a SUBSET of block 1's {missing_section, missing_verdict}, i.e. the same defect partially repaired, so the second consecutive block MUST still escalate. Suppressing here is the cycle-1289 audit defect (whole-string compare of summarize() output).",
			probe.dispatched[2])
	}
}

// TestContractCorrection_SupersetRegressionStillEscalates is the subset case's
// mirror: the correction not only failed to close block 1's violation, it added
// a second one. A SUPERSET is at least as strong an incapable-CLI signature as a
// subset, and whole-string identity suppresses it for the identical reason.
func TestContractCorrection_SupersetRegressionStillEscalates(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_verdict] no VERDICT sentinel line found",
			"build deliverable failed contract: [missing_verdict] no VERDICT sentinel line found; [missing_section] required section 'Findings' not found",
		},
	}
	if probe.reasonPerBlock[0] == probe.reasonPerBlock[1] {
		t.Fatal("fixture invalid: the two reasons must differ VERBATIM")
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — block 2's codes {missing_verdict, missing_section} are a SUPERSET of block 1's {missing_verdict}: the original violation is still open and the correction added another, so the second consecutive block MUST still escalate",
			probe.dispatched[2])
	}
}

// TestContractCorrection_DisjointViolationSetsDoNotEscalate is the NEGATIVE
// axis, and the guard against "fix the subset case by escalating on everything".
// Two MULTI-violation blocks sharing NO code are two honest, unrelated defects —
// scoping constraint 4's original intent — so correction 2 must re-dispatch on
// the phase's own routing (empty ModelRoutingCLI override), exactly as before
// the escalation feature landed. An implementation that simply deleted the
// identity gate to make the subset test pass fails here.
func TestContractCorrection_DisjointViolationSetsDoNotEscalate(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_section] required section 'Findings' not found; [empty_artifact] deliverable is zero bytes",
			"build deliverable failed contract: [missing_verdict] no VERDICT sentinel line found; [bad_trailer] Evolve-Phase trailer malformed",
		},
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"\" — block 1's codes {missing_section, empty_artifact} and block 2's {missing_verdict, bad_trailer} are DISJOINT, so these are two honest defects and escalation must NOT fire (scoping constraint 4)",
			probe.dispatched[2])
	}
}

// TestContractCorrection_ReorderedViolationSetEscalates is the EDGE axis on
// rendering instability. summarize() joins violations in slice order and quotes
// their messages verbatim, so ONE defect set can render two ways across blocks
// (different order, reworded message text). Code-SET identity is invariant to
// both; any residual text-shaped comparison is not.
func TestContractCorrection_ReorderedViolationSetEscalates(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{
		phase:     "build",
		threshold: neverDemotingThreshold,
		reasonPerBlock: []string{
			"build deliverable failed contract: [missing_section] required section 'Findings' not found; [missing_verdict] no VERDICT sentinel line found",
			"build deliverable failed contract: [missing_verdict] VERDICT sentinel line is absent; [missing_section] section 'Findings' is missing",
		},
	}
	if probe.reasonPerBlock[0] == probe.reasonPerBlock[1] {
		t.Fatal("fixture invalid: the two reasons must differ VERBATIM")
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — both blocks carry the SAME code set {missing_section, missing_verdict}, differing only in violation ORDER and message wording, so the second consecutive block MUST still escalate",
			probe.dispatched[2])
	}
}

// TestContractCorrection_UncodedReasonsFallBackToTextIdentity is the fail-safe
// EDGE axis. Not every rejection reason on this path is a summarize() rendering
// carrying "[code]" tokens, and a code-set implementation that extracts ZERO
// codes from both blocks must not conclude "empty ∩ empty = ∅ ⇒ different
// defect" — that would silently delete the escalation ladder for every
// non-summarize reason shape. Two IDENTICAL code-less reasons are self-evidently
// the same defect and must still escalate.
func TestContractCorrection_UncodedReasonsFallBackToTextIdentity(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	const uncoded = "build deliverable failed contract: artifact absent at the contracted path"
	probe := &escalationProbe{
		phase:          "build",
		threshold:      neverDemotingThreshold,
		reasonPerBlock: []string{uncoded, uncoded},
	}
	if strings.Contains(uncoded, "[") {
		t.Fatal("fixture invalid: the reason must carry NO bracketed violation code")
	}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3 (initial + 2 corrections)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"codex-tmux\" — both blocks report the IDENTICAL code-less reason %q, so they are the same defect; a code-set gate that reads two empty sets as \"different\" would delete the ladder for every non-summarize reason shape",
			probe.dispatched[2], uncoded)
	}
}
