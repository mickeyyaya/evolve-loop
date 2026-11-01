package core

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

type escalationProbe struct {
	phase          string
	compliantCLI   string
	threshold      int
	approveAfter   int
	reasonPerBlock []string

	lastCLI    string
	dispatched []string
	blocks     int
	reviews    int
}

func (p *escalationProbe) Name() string { return p.phase }

func (p *escalationProbe) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.lastCLI = req.Driver.String()
	p.dispatched = append(p.dispatched, req.Driver.String())
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

const neverDemotingThreshold = 99

func TestContractCorrection_SecondBlockEscalatesToProfileFallback(t *testing.T) {
	t.Parallel()

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
		t.Errorf("initial dispatch carried Driver=%q, want \"\" — the phase's PRIMARY routing must be untouched (scoping constraint: escalate only the re-dispatch that already failed the contract)", probe.dispatched[0])
	}
	if probe.dispatched[2] != "codex-tmux" {
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — the SECOND consecutive contract block must re-dispatch on the profile's cli_fallback BEFORE the third strike opens the circuit", probe.dispatched[2])
	}
}

func TestContractCorrection_FirstBlockDoesNotEscalate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold, approveAfter: 1}

	if _, err := runEscalationCycle(t, root, probe); err != nil {
		t.Fatalf("RunCycle should proceed after one correction: %v", err)
	}
	if len(probe.dispatched) != 2 {
		t.Fatalf("build dispatched %d times (%v), want 2 (initial + 1 correction)", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[1] != "" {
		t.Errorf("correction 1 dispatched on Driver=%q, want \"\" \u2014 the FIRST contract block must NOT escalate the CLI (one bad turn is not a CLI verdict)", probe.dispatched[1])
	}
}

func TestContractCorrection_HotBreakerEscalatesOnFirstCorrection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: 3, blocks: 1}

	led, err := runEscalationCycle(t, root, probe)
	if err != nil {
		t.Fatalf("the third block demotes and therefore approves: %v", err)
	}
	if len(probe.dispatched) < 2 {
		t.Fatalf("build dispatched %v, want at least initial + 1 correction", probe.dispatched)
	}
	if probe.dispatched[1] != "codex-tmux" {
		t.Errorf("correction 1 dispatched on Driver=%q, want \"codex-tmux\" \u2014 with the breaker already at 1, this phase's first block IS the second consecutive block", probe.dispatched[1])
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

func TestContractCorrection_NonContractRejectionNeverEscalates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}
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
			t.Errorf("dispatch %d carried Driver=%q, want \"\" \u2014 a non-contract rejection must never escalate the CLI (dispatched=%v)", i, cli, probe.dispatched)
		}
	}
}

func TestContractCorrection_NoDeclaredFallbackEscalatesToUniversalClaude(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "claude-tmux" {
		t.Errorf("correction 2 dispatched on Driver=%q, want %q — a profile with no declared cli_fallback must escalate to the universal claude fallback", probe.dispatched[2], "claude-tmux")
	}
}

func TestContractCorrection_SameFamilyFallbackIsNotAnEscalation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", nil)
	probe := &escalationProbe{phase: "build", threshold: neverDemotingThreshold}

	if _, err := runEscalationCycle(t, root, probe); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want 3", len(probe.dispatched), probe.dispatched)
	}
	if probe.dispatched[2] != "" {
		t.Errorf("correction 2 dispatched on Driver=%q, want \"\" — a primary already on the universal-fallback family has NO escalation target; re-pointing at the same family is not an escalation", probe.dispatched[2])
	}
}

func TestContractCorrection_CompliantFallbackPreventsCircuitOpen(t *testing.T) {
	t.Parallel()

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

func TestContractCorrection_CircuitOpenWarnsAndFilesItem(t *testing.T) {
	t.Parallel()

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

func TestFormatContractGateDemotionWarn(t *testing.T) {
	t.Parallel()
	reason := "triage deliverable failed contract: [missing_failure_block] schema_version 2 required"
	got := formatContractGateDemotionWarn("triage", "agy-tmux", contractDispatch{escalated: true}, reason)
	for _, want := range []string{"WARN", "CONTRACT GATE DEMOTED", "triage", "agy-tmux", "advisory", "missing_failure_block"} {
		if !strings.Contains(got, want) {
			t.Errorf("demotion WARN missing %q: %s", want, got)
		}
	}
	if !strings.Contains(got, "escalation already ran") {
		t.Errorf("escalated=true must SAY escalation ran: %s", got)
	}
	notTried := formatContractGateDemotionWarn("triage", "agy-tmux", contractDispatch{}, reason)
	if !strings.Contains(notTried, "did NOT run") {
		t.Errorf("escalated=false must NOT claim escalation was attempted: %s", notTried)
	}
}

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

func TestUniversalContractFallbackMatchesLLMRouteDefault(t *testing.T) {
	t.Parallel()
	plan := llmroute.Resolve("builder", "build", "balanced", nil, llmroute.AgentOverride{}, nil, nil, nil)
	if len(plan.Candidates) == 0 {
		t.Fatal("llmroute.Resolve returned no candidates")
	}
	if plan.Candidates[0] != "claude-tmux" {
		t.Errorf("the contract-escalation seat %q differs from llmroute's no-profile default %q — the escalation target must be the same universal fallback the dispatch resolver uses", "claude-tmux", plan.Candidates[0])
	}
}

func TestContractCorrection_DifferingBlockReasonsDoNotEscalate(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"\" — block 2's violation (%q) DIFFERS from block 1's (%q), so the two blocks are not one incapable-CLI signature and escalation must not fire",
			probe.dispatched[2], probe.reasonPerBlock[1], probe.reasonPerBlock[0])
	}
}

func TestContractCorrection_NormalizedIdenticalReasonsEscalate(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — the two blocks are the SAME defect under normalizeReasonForFingerprint (they differ only in a duration token), so the second consecutive block must still escalate",
			probe.dispatched[2])
	}
}

func TestContractCorrection_SubsetRepairStillEscalates(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — block 2's violation codes {missing_verdict} are a SUBSET of block 1's {missing_section, missing_verdict}, i.e. the same defect partially repaired, so the second consecutive block MUST still escalate. Suppressing here is the cycle-1289 audit defect (whole-string compare of summarize() output).",
			probe.dispatched[2])
	}
}

func TestContractCorrection_SupersetRegressionStillEscalates(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — block 2's codes {missing_verdict, missing_section} are a SUPERSET of block 1's {missing_verdict}: the original violation is still open and the correction added another, so the second consecutive block MUST still escalate",
			probe.dispatched[2])
	}
}

func TestContractCorrection_DisjointViolationSetsDoNotEscalate(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"\" — block 1's codes {missing_section, empty_artifact} and block 2's {missing_verdict, bad_trailer} are DISJOINT, so these are two honest defects and escalation must NOT fire (scoping constraint 4)",
			probe.dispatched[2])
	}
}

func TestContractCorrection_ReorderedViolationSetEscalates(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — both blocks carry the SAME code set {missing_section, missing_verdict}, differing only in violation ORDER and message wording, so the second consecutive block MUST still escalate",
			probe.dispatched[2])
	}
}

func TestContractCorrection_UncodedReasonsFallBackToTextIdentity(t *testing.T) {
	t.Parallel()

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
		t.Errorf("correction 2 dispatched on Driver=%q, want \"codex-tmux\" — both blocks report the IDENTICAL code-less reason %q, so they are the same defect; a code-set gate that reads two empty sets as \"different\" would delete the ladder for every non-summarize reason shape",
			probe.dispatched[2], uncoded)
	}
}

func TestContractArtifactDetermined_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		code string
		want bool
	}{
		{"missing_artifact", "missing_artifact", true},
		{"empty_artifact", "empty_artifact", true},
		{"missing_section", "missing_section", true},
		{"bad_verdict", "bad_verdict", true},
		{"missing_key", "missing_key", true},
		{"missing_challenge_token", "missing_challenge_token", true},

		{"stray_in_worktree", "stray_in_worktree", false},

		{"invalid_json", "invalid_json", false},
		{"failure_context_missing", "failure_context_missing", false},

		{"unknown_code", "totally_unknown_future_code", false},
		{"empty_string", "", false},
		{"case_variant_is_not_the_code", "Missing_Section", false},
		{"bracketed_rendering_is_not_a_code", "[missing_section]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := contractArtifactDetermined(tc.code); got != tc.want {
				t.Errorf("contractArtifactDetermined(%q) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestContractArtifactDetermined_StrayInWorktreeIsFalse(t *testing.T) {
	t.Parallel()
	if contractArtifactDetermined("stray_in_worktree") {
		t.Error("contractArtifactDetermined(\"stray_in_worktree\") = true, want false — stray_in_worktree is repaired by deleting a worktree copy, which leaves the watched artifact byte-identical; treating it as artifact-determined lets an unchanged hash abort a repairable ladder (inst-L1508a)")
	}
	if !contractArtifactDetermined("missing_section") {
		t.Error("contractArtifactDetermined(\"missing_section\") = false, want true — repairing a missing section edits the artifact, so hash equality IS sound evidence there")
	}
}

func TestContractArtifactDetermined_CodesMatchDeliverableVocabulary(t *testing.T) {
	t.Parallel()
	allDeliverableCodes := []string{
		"missing_artifact", "empty_artifact", "missing_section",
		"missing_challenge_token", "bad_verdict", "stray_in_worktree",
		"invalid_json", "missing_key", "failure_context_missing",
	}
	determined := 0
	for _, code := range allDeliverableCodes {
		if contractArtifactDetermined(code) {
			determined++
		}
	}
	if determined != 6 {
		t.Errorf("contractArtifactDetermined accepts %d of the %d real deliverable codes, want exactly 6 — the allowlist must be neither widened past the artifact-determined six nor narrowed below them", determined, len(allDeliverableCodes))
	}
}
