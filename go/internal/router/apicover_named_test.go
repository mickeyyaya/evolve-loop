package router

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestDefaultShipFloor_IsTddBuildAudit(t *testing.T) {
	got := DefaultShipFloor()
	want := []string{"tdd", "build", "audit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultShipFloor() = %v, want %v", got, want)
	}
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

func TestFailureInsertPhases_AndIsFailureInsert(t *testing.T) {
	got := FailureInsertPhases()
	want := []string{"bug-reproduction", "fault-localization"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FailureInsertPhases() = %v, want %v (sorted)", got, want)
	}
	for _, p := range got {
		if !IsFailureInsert(p) {
			t.Errorf("IsFailureInsert(%q) = false, want true (it is in FailureInsertPhases)", p)
		}
	}
	if IsFailureInsert("build") {
		t.Errorf("IsFailureInsert(%q) = true, want false (build is not a failure insert)", "build")
	}
}

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

func TestRoutingSignals_HasEmptyTriageCommitment(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-triage.json", `{"cycle_size":"small"}`)
	writeFile(t, ws, "triage-decision.json", `{"top_n":[]}`)
	sig, err := Digest(ws, []string{"triage"})
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !sig.HasEmptyTriageCommitment() {
		t.Error("HasEmptyTriageCommitment() = false, want true for an explicit empty top_n")
	}
}

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

	// A ship plan makes the clamp rewrite the entries around the mint.
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

func TestPhasePolicy_ProducerAndEnabled(t *testing.T) {
	cfg := testCfg()
	cfg.PhaseEnable["plan-review"] = config.EnableOff
	p := NewPhasePolicy(cfg)
	// The explicit binding names PhasePolicy in the test AST, apicover's coverage signal.
	var _ PhasePolicy = p
	if !p.Enabled("build", RoutingSignals{}) {
		t.Errorf("PhasePolicy.Enabled(build) = false, want true (mandatory)")
	}
	if p.Enabled("plan-review", RoutingSignals{}) {
		t.Errorf("PhasePolicy.Enabled(plan-review) = true, want false (forced off)")
	}
	if !p.ShouldRunPhase("build") || p.ShouldRunPhase("plan-review") {
		t.Errorf("ShouldRunPhase: build=%v plan-review=%v, want true/false",
			p.ShouldRunPhase("build"), p.ShouldRunPhase("plan-review"))
	}
}

func TestRoutingStrategy_Satisfaction(t *testing.T) {
	var staticS RoutingStrategy = StaticPreset{}
	var llmS RoutingStrategy = LLMProposal{Proposer: nil}

	in := base("start")
	if d := staticS.Decide(in); d.NextPhase != "scout" {
		t.Errorf("StaticPreset.Decide(start).NextPhase = %q, want scout", d.NextPhase)
	}
	if d := llmS.Decide(in); d.NextPhase != "scout" {
		t.Errorf("LLMProposal.Decide(start).NextPhase = %q, want scout (kernel floor)", d.NextPhase)
	}
	got := Select(config.RoutingConfig{Mode: config.ModeDynamicLLM}, nil)
	if _, ok := got.(StaticPreset); !ok {
		t.Errorf("Select(DynamicLLM, nil) = %T, want StaticPreset fallback", got)
	}
}

type fakePlanner struct{ plan *PhasePlan }

func (f fakePlanner) Plan(in RouteInput) (*PhasePlan, error) { return f.plan, nil }

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
	out, _ := ClampPlanToFloor(nonTrivialIn(), got)
	if len(out.MintPhases) != 1 || out.MintPhases[0].Name != "perf-bench" {
		t.Errorf("MintPhases not carried through clamp: %+v", out.MintPhases)
	}
}
