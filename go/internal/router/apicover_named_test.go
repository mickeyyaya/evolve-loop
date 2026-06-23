package router

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage. Each test
// names a previously-uncovered exported symbol and exercises it through its
// REAL producer/consumer (no `_ = pkg.X` padding): funcs are invoked and
// asserted; types are bound via route assembly / the strategy engine / a real
// digest projection and then read back. Conventions match the existing suite
// (testCfg, writeFile, Digest fixtures, the StaticPreset/LLMProposal brains).

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
)

// TestDefaultShipFloor_IsTddBuildAudit invokes DefaultShipFloor and asserts the
// safe structural default the router owns: tdd→build→audit, in that order.
func TestDefaultShipFloor_IsTddBuildAudit(t *testing.T) {
	got := DefaultShipFloor()
	want := []string{"tdd", "build", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultShipFloor() = %v, want %v", got, want)
	}
	// It must drive the back-compat clamp: a ship plan forces exactly this set on.
	out, clamps := ClampPlanToFloor(nonTrivialIn(), &PhasePlan{
		Entries: []PhasePlanEntry{pe("scout", true), pe("ship", true)},
	})
	for _, phase := range want {
		if !planRuns(out, phase) {
			t.Errorf("DefaultShipFloor phase %q not forced to run by ClampPlanToFloor", phase)
		}
		if !clampsHave(clamps, "ship-requires-"+phase) {
			t.Errorf("missing ship-requires-%s clamp; clamps=%+v", phase, clamps)
		}
	}
}

// TestFailureInsertPhases_AndIsFailureInsert invokes both functions: the slice
// is the sorted retry-path insert names, and IsFailureInsert is the membership
// predicate over that same one home of the belief.
func TestFailureInsertPhases_AndIsFailureInsert(t *testing.T) {
	got := FailureInsertPhases()
	want := []string{"bug-reproduction", "fault-localization"} // sorted
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FailureInsertPhases() = %v, want %v (sorted)", got, want)
	}
	// IsFailureInsert agrees with the slice it is rendered from.
	for _, p := range got {
		if !IsFailureInsert(p) {
			t.Errorf("IsFailureInsert(%q) = false, want true (it is in FailureInsertPhases)", p)
		}
	}
	if IsFailureInsert("build") {
		t.Errorf("IsFailureInsert(%q) = true, want false (build is not a failure insert)", "build")
	}
}

// TestHandoffsFromSignals_ProjectsDigest binds the typed phaseio views from a
// REAL digest (Digest over on-disk handoff fixtures) and asserts the projection
// — including the Severity-ordinal → canonical-word mapping that is the only
// non-trivial conversion. Producer-driven, not a bare literal.
func TestHandoffsFromSignals_ProjectsDigest(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	writeFile(t, ws, "handoff-auditor.json", auditHandoff)
	writeFile(t, ws, "handoff-scout.json", scoutHandoff)

	sig, err := Digest(ws, []string{"scout", "build", "audit"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	h := HandoffsFromSignals(sig)

	sc, ok := h.Scout()
	if !ok || sc.CycleSizeEstimate != sig.Scout.CycleSizeEstimate || sc.ItemCount != sig.Scout.ItemCount {
		t.Errorf("Scout view = (%+v, ok=%v), want digest %+v", sc, ok, sig.Scout)
	}
	b, ok := h.Build()
	if !ok || b.Verdict != sig.Build.Verdict || b.ACSRed != sig.Build.ACSRed {
		t.Errorf("Build view = (%+v, ok=%v), want digest %+v", b, ok, sig.Build)
	}
	if b.SeverityMax != sig.Build.SeverityMax.String() {
		t.Errorf("Build.SeverityMax = %q, want word form %q (ordinal→word)", b.SeverityMax, sig.Build.SeverityMax.String())
	}
	a, ok := h.Audit()
	if !ok || a.RedCount != sig.Audit.RedCount || a.Confidence != sig.Audit.Confidence {
		t.Errorf("Audit view = (%+v, ok=%v), want digest %+v", a, ok, sig.Audit)
	}
	if a.DefectsBySeverity["MEDIUM"] != sig.Audit.DefectsBySeverity[SevMedium] {
		t.Errorf("Audit.DefectsBySeverity word-keyed[MEDIUM]=%d, want ordinal %d",
			a.DefectsBySeverity["MEDIUM"], sig.Audit.DefectsBySeverity[SevMedium])
	}
}

// TestRouteInput_AdvisorContextTypes binds the four advisor-context value types
// — BenchedCLI, CarryoverTodo, PhaseCard, MintSpec — onto a RouteInput/PhasePlan
// exactly as the orchestrator threads them, then reads the fields back. MintSpec
// is carried INTO a PhasePlanEntry and proven to survive the floor clamp (which
// governs run/skip, never the minted-phase payload).
func TestRouteInput_AdvisorContextTypes(t *testing.T) {
	until := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	writesSrc := true
	mint := &MintSpec{Prompt: "audit the migration", Tier: "deep", CLI: "claude-tmux", WritesSource: &writesSrc}

	in := RouteInput{
		Current:        "scout",
		Cfg:            testCfg(),
		Now:            fixedTime(),
		BenchedCLIs:    []BenchedCLI{{Family: "codex", Reason: "rate_limit", Until: until}},
		CarryoverTodos: []CarryoverTodo{{ID: "t1", Action: "wire flag", Priority: "high", FirstSeenCycle: 40, CyclesUnpicked: 2}},
		Catalog: []PhaseCard{{
			Name: "security-scan", Role: "evaluate", Tier: "deep",
			WritesSource: false, Optional: true, Description: "scans the diff",
			WhenToUse: "auth/input code", Categories: []string{"security"},
		}},
	}

	// BenchedCLI / CarryoverTodo / PhaseCard are advisor-context DTOs: the router
	// carries them in RouteInput for the (out-of-package) advisor to serialize into
	// the LLM proposer's environmental context — Route()/ClampPlanToFloor never read
	// them. Their real contract is the WIRE shape, so exercise encoding/json with
	// each type's actual tags rather than a set-then-read (a tag drift fails here).

	// CarryoverTodo: the snake_case keys the advisor prompt exposes must bind, and
	// the value must survive a full marshal->unmarshal round-trip unchanged.
	ctBlob, err := json.Marshal(in.CarryoverTodos[0])
	if err != nil {
		t.Fatalf("marshal CarryoverTodo: %v", err)
	}
	for _, key := range []string{`"id":"t1"`, `"priority":"high"`, `"first_seen_cycle":40`, `"cycles_unpicked":2`} {
		if !strings.Contains(string(ctBlob), key) {
			t.Errorf("CarryoverTodo JSON missing %s; got %s", key, ctBlob)
		}
	}
	var ct CarryoverTodo
	if err := json.Unmarshal(ctBlob, &ct); err != nil || ct != in.CarryoverTodos[0] {
		t.Errorf("CarryoverTodo round-trip = %+v (err %v), want %+v", ct, err, in.CarryoverTodos[0])
	}

	// PhaseCard: the SELECT-hint keys the advisor renders, and the writes_source
	// omitempty contract (absent when false, so a card cannot imply it mutates source).
	pcBlob, err := json.Marshal(in.Catalog[0])
	if err != nil {
		t.Fatalf("marshal PhaseCard: %v", err)
	}
	for _, key := range []string{`"name":"security-scan"`, `"role":"evaluate"`, `"optional":true`, `"when_to_use":"auth/input code"`, `"categories":["security"]`} {
		if !strings.Contains(string(pcBlob), key) {
			t.Errorf("PhaseCard JSON missing %s; got %s", key, pcBlob)
		}
	}
	if strings.Contains(string(pcBlob), "writes_source") {
		t.Errorf("PhaseCard JSON must omit writes_source when false (omitempty); got %s", pcBlob)
	}

	// BenchedCLI carries no json tags (environmental context); assert the carried
	// data survives a marshal->unmarshal round-trip (the form the advisor consumes).
	bcBlob, err := json.Marshal(in.BenchedCLIs[0])
	if err != nil {
		t.Fatalf("marshal BenchedCLI: %v", err)
	}
	var bc BenchedCLI
	if err := json.Unmarshal(bcBlob, &bc); err != nil {
		t.Fatalf("unmarshal BenchedCLI: %v", err)
	}
	if bc.Family != "codex" || bc.Reason != "rate_limit" || !bc.Until.Equal(until) {
		t.Errorf("BenchedCLI round-trip = %+v, want family=codex reason=rate_limit until=%v", bc, until)
	}

	// MintSpec — carried on a plan entry; the floor clamp must preserve it
	// (it governs run/skip, not the minted payload). Use a ship plan so the
	// clamp actually rewrites entries around it.
	plan := &PhasePlan{Entries: []PhasePlanEntry{
		pe("scout", true),
		{Phase: "security-scan", Run: true, Mint: mint},
		pe("ship", true),
	}}
	out, _ := ClampPlanToFloorWith(in, plan, DefaultShipFloor(), false)
	var gotMint *MintSpec
	for _, e := range out.Entries {
		if e.Phase == "security-scan" {
			gotMint = e.Mint
		}
	}
	if gotMint == nil {
		t.Fatalf("MintSpec entry dropped by clamp; out=%+v", out.Entries)
	}
	if gotMint.Prompt != "audit the migration" || gotMint.Tier != "deep" || gotMint.CLI != "claude-tmux" {
		t.Errorf("MintSpec survived = %+v, want prompt/tier=deep/cli=claude-tmux preserved", gotMint)
	}
	if gotMint.WritesSource == nil || *gotMint.WritesSource != true {
		t.Errorf("MintSpec.WritesSource tri-state lost: %+v", gotMint.WritesSource)
	}
}

// TestPhasePolicy_ProducerAndEnabled binds PhasePolicy through its producer
// NewPhasePolicy and exercises the enablement decision: a mandatory phase runs,
// a forced-off phase does not. (policy_test.go covers the full matrix; this
// asserts the type itself answers via its real entry point.)
func TestPhasePolicy_ProducerAndEnabled(t *testing.T) {
	cfg := testCfg()
	cfg.PhaseEnable["plan-review"] = config.EnableOff
	// Explicit type binding so the PhasePolicy identifier is named in the test AST
	// (apicover's coverage signal) and is the real producer's return type.
	var p PhasePolicy = NewPhasePolicy(cfg)

	if !p.Enabled("build", RoutingSignals{}) {
		t.Errorf("PhasePolicy.Enabled(build) = false, want true (mandatory)")
	}
	if p.Enabled("plan-review", RoutingSignals{}) {
		t.Errorf("PhasePolicy.Enabled(plan-review) = true, want false (forced off)")
	}
	// ShouldRunPhase is the self-skipping authority (Enforce stage defers to Enabled).
	if !p.ShouldRunPhase("build") || p.ShouldRunPhase("plan-review") {
		t.Errorf("ShouldRunPhase: build=%v plan-review=%v, want true/false",
			p.ShouldRunPhase("build"), p.ShouldRunPhase("plan-review"))
	}
}

// TestRoutingStrategy_Satisfaction proves the concrete brains satisfy the
// RoutingStrategy interface and exercises a method on each — the static preset
// and the dynamic LLM proposal both Decide() down to the same kernel floor.
func TestRoutingStrategy_Satisfaction(t *testing.T) {
	var staticS RoutingStrategy = StaticPreset{}
	var llmS RoutingStrategy = LLMProposal{Proposer: nil} // nil proposer ⇒ degrades to kernel

	in := base("start")
	if d := staticS.Decide(in); d.NextPhase != "scout" {
		t.Errorf("StaticPreset.Decide(start).NextPhase = %q, want scout", d.NextPhase)
	}
	// LLMProposal with a nil proposer must produce the identical kernel decision.
	if d := llmS.Decide(in); d.NextPhase != "scout" {
		t.Errorf("LLMProposal.Decide(start).NextPhase = %q, want scout (kernel floor)", d.NextPhase)
	}
	// Select wires the concrete type from config; a DynamicLLM mode with no
	// proposer falls back to StaticPreset.
	got := Select(config.RoutingConfig{Mode: config.ModeDynamicLLM}, nil)
	if _, ok := got.(StaticPreset); !ok {
		t.Errorf("Select(DynamicLLM, nil) = %T, want StaticPreset fallback", got)
	}
}

// fakePlanner is a concrete Planner used to prove interface satisfaction and to
// exercise the Plan method (the concrete LLM planner lives in package core; the
// router only defines the interface it consumes).
type fakePlanner struct{ plan *PhasePlan }

func (f fakePlanner) Plan(in RouteInput) (*PhasePlan, error) { return f.plan, nil }

// TestPlanner_SatisfactionAndExercise binds a value to the Planner interface and
// exercises Plan(), asserting the returned whole-cycle plan flows back. The plan
// is then clamped, proving the produced PhasePlan is the kernel's real input.
func TestPlanner_SatisfactionAndExercise(t *testing.T) {
	want := &PhasePlan{
		Entries: []PhasePlanEntry{pe("scout", true), pe("build", true), pe("ship", true)},
		MintPhases: []phaseconfig.PhaseConfig{
			{PhaseSpec: phasespec.PhaseSpec{Name: "perf-bench"}},
		},
	}
	var pl Planner = fakePlanner{plan: want}

	got, err := pl.Plan(base("start"))
	if err != nil {
		t.Fatalf("Planner.Plan: %v", err)
	}
	if got != want {
		t.Fatalf("Planner.Plan returned %p, want the produced plan %p", got, want)
	}
	// MintPhases must carry through the floor clamp untouched.
	out, _ := ClampPlanToFloor(nonTrivialIn(), got)
	if len(out.MintPhases) != 1 || out.MintPhases[0].Name != "perf-bench" {
		t.Errorf("MintPhases not carried through clamp: %+v", out.MintPhases)
	}
}
