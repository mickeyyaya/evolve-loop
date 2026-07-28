package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Cycle-1155 RED contract — replan-rejections-telemetry.
//
// The enforcement half is already shipped: ClampPlanToFloorWith drops
// unknown-phase entries on BOTH the upfront (cyclerun.go) and the post-scout
// re-plan (cyclerun_replan.go) path. The telemetry half is not:
// router.ValidatePlan has exactly one call site (cyclerun.go:676), so a re-plan
// that hallucinates a phase is dropped SILENTLY — zero forensic trail, which is
// precisely what dropUnknownPhases' own doc comment (floor.go:151) exists to
// close. Compounding it, recordPlanRejections writes advisor-rejections.json
// UNCONDITIONALLY, so a naive second call site would overwrite the upfront
// record instead of accumulating.
//
// These tests pin BEHAVIOUR, not a file format. The builder is free to
// accumulate as a kind-keyed object in advisor-rejections.json, as sibling
// advisor-rejections-<kind>.json files, or any other shape — the assertions go
// through collectWorkspaceRejections/replanRecordPresent, which read every
// advisor-rejections*.json in the workspace and recover rejections from any
// nesting, attributing each to a plan-kind via the filename and the JSON keys on
// its path. What is NOT negotiable: a re-plan's rejections must be recoverable
// and attributable to the re-plan, and recording them must not destroy the
// upfront plan's record.

// phantomReplanPhase / phantomReplanPhase2 are names no known-phase channel can
// supply: not canonical, not in Cfg.Order/Mandatory/Triggers/Conditional, not in
// the catalog, and not minted by the plan (no Mint on the entry). ValidatePlan
// must therefore classify them "unknown-phase".
const (
	phantomReplanPhase  = "phantom-replan-1155"
	phantomReplanPhase2 = "phantom-replan-1155-b"
	// seededInitialPhase stands in for a rejection the UPFRONT plan already
	// recorded before the re-plan runs (the shape recordPlanRejections writes
	// today: a flat array in advisor-rejections.json). It must survive.
	seededInitialPhase = "phantom-initial-1155"
)

// recordedRejection is one rejection recovered from workspace telemetry, plus
// the kind hint (lowercased filename + JSON key path) it was found under.
type recordedRejection struct {
	Kind   string
	Phase  string
	Reason string
}

// collectWorkspaceRejections reads every advisor-rejections*.json under ws and
// recovers every rejection object in it, regardless of nesting or key naming.
func collectWorkspaceRejections(t *testing.T, ws string) []recordedRejection {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(ws, "advisor-rejections*.json"))
	if err != nil {
		t.Fatalf("glob advisor-rejections*.json: %v", err)
	}
	var out []recordedRejection
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s is not valid json: %v", filepath.Base(p), err)
		}
		walkRejections(doc, strings.ToLower(filepath.Base(p)), &out)
	}
	return out
}

func walkRejections(node any, hint string, out *[]recordedRejection) {
	switch v := node.(type) {
	case map[string]any:
		if r, ok := asRejection(v); ok {
			r.Kind = hint
			*out = append(*out, r)
			return
		}
		for k, child := range v {
			walkRejections(child, hint+"/"+strings.ToLower(k), out)
		}
	case []any:
		for _, child := range v {
			walkRejections(child, hint, out)
		}
	}
}

// asRejection recognises a router.PlanRejection however it was keyed (Go's
// default exported-field casing or an explicit lowercase json tag).
func asRejection(m map[string]any) (recordedRejection, bool) {
	pick := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := m[k].(string); ok {
				return s
			}
		}
		return ""
	}
	reason := pick("Reason", "reason")
	if reason == "" {
		return recordedRejection{}, false
	}
	return recordedRejection{Phase: pick("Phase", "phase"), Reason: reason}, true
}

// replanRecordPresent reports whether the workspace holds a rejection record
// ATTRIBUTED TO THE RE-PLAN — a file whose name says "replan" or a JSON key that
// does. It is deliberately satisfied by an EMPTY finding set: recordPlanRejections'
// contract is that "[]" means validated-clean, distinct from "validation never
// ran", so a clean re-plan must still leave proof that ValidatePlan ran on it.
func replanRecordPresent(t *testing.T, ws string) bool {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(ws, "advisor-rejections*.json"))
	if err != nil {
		t.Fatalf("glob advisor-rejections*.json: %v", err)
	}
	for _, p := range paths {
		if strings.Contains(strings.ToLower(filepath.Base(p)), "replan") {
			return true
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s is not valid json: %v", filepath.Base(p), err)
		}
		if hasReplanKey(doc) {
			return true
		}
	}
	return false
}

func hasReplanKey(node any) bool {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if strings.Contains(strings.ToLower(k), "replan") || hasReplanKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if hasReplanKey(child) {
				return true
			}
		}
	}
	return false
}

func hasRejection(rejs []recordedRejection, phase, reason string, fromReplan bool) bool {
	for _, r := range rejs {
		if r.Phase != phase || r.Reason != reason {
			continue
		}
		if !fromReplan || strings.Contains(r.Kind, "replan") {
			return true
		}
	}
	return false
}

// seqReplanPlanner returns a DIFFERENT plan on each RePlan call, so a cycle with
// RePlanMaxDepth > 1 produces distinguishable per-re-plan rejection sets.
type seqReplanPlanner struct {
	plans  []*router.PhasePlan
	calls  int
	static *router.PhasePlan
}

func (p *seqReplanPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return p.static, nil
}

func (p *seqReplanPlanner) RePlan(router.RouteInput) (*router.PhasePlan, error) {
	if p.calls >= len(p.plans) {
		return nil, nil
	}
	plan := p.plans[p.calls]
	p.calls++
	return plan, nil
}

// replanRejOrchestrator mirrors replanOrchestrator (cyclerun_replan_test.go) but
// takes the re-plan depth cap, so the multi-re-plan accumulation case is
// expressible. Shadow stage: the re-plan is RECORDED, never swapped in.
func replanRejOrchestrator(t *testing.T, pl router.Planner, maxDepth int) *Orchestrator {
	t.Helper()
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.RouterReplan = config.StageShadow
	cfg.RePlanMaxDepth = maxDepth
	cfg.Triggers = map[string]config.RoutingBlock{
		"tester": {InsertWhen: []config.Condition{{Field: "scout.item_count", Op: "gte", Value: 1}}},
	}
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRouting(cfg, router.StaticPreset{}), WithPlanner(pl))
}

func replanRejCycleRun(t *testing.T, ws string, o *Orchestrator, cycle int) *cycleRun {
	t.Helper()
	return &cycleRun{
		o: o, ctx: context.Background(),
		req: CycleRequest{ProjectRoot: ws}, cycle: cycle,
		cs:      CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap: map[string]string{},
		// Omits "tester", which the measured signals (item_count=2) trigger — the
		// material divergence that makes postScoutReplan actually re-plan.
		clampedPlan: &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}},
	}
}

func knownPhasePlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

func planWithPhantom(phantom string) *router.PhasePlan {
	p := knownPhasePlan()
	p.Entries = append(p.Entries, router.PhasePlanEntry{Phase: phantom, Run: true})
	return p
}

// TestReplan_UnknownPhase_RecordsRejection — AC1 (the crux). A post-scout re-plan
// naming a phase outside the known set must leave a forensic record: an
// "unknown-phase" rejection for that phase, attributable to the re-plan. Today
// the clamp drops it silently and no rejection is recorded anywhere, so this is
// RED. The cheapest fake — recording the CLAMPED plan's rejections — records
// nothing (the clamp already removed the phantom), so it cannot pass:
// ValidatePlan must run on the RAW re-plan, pre-clamp.
func TestReplan_UnknownPhase_RecordsRejection(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	pl := &seqReplanPlanner{plans: []*router.PhasePlan{planWithPhantom(phantomReplanPhase)}}
	cr := replanRejCycleRun(t, ws, replanRejOrchestrator(t, pl, 1), 11)

	cr.postScoutReplan()

	if pl.calls != 1 {
		t.Fatalf("RePlan called %d times, want 1 (the mismatch gate must fire)", pl.calls)
	}
	rejs := collectWorkspaceRejections(t, ws)
	if !hasRejection(rejs, phantomReplanPhase, "unknown-phase", true) {
		t.Errorf("re-plan naming unknown phase %q recorded no attributable unknown-phase rejection; recovered=%+v", phantomReplanPhase, rejs)
	}
	// The drop itself must still happen — telemetry is report-only and must not
	// have been bought by weakening the floor.
	if planEntryRuns(cr.clampedPlan, phantomReplanPhase) {
		t.Errorf("the integrity floor must still DROP %q from the plan", phantomReplanPhase)
	}
}

// TestReplan_KnownPhasesOnly_NoSpuriousRejection — AC2 (negative + no-data-loss).
// A clean re-plan must (a) record NO unknown-phase rejection, (b) still leave
// proof that validation RAN on the re-plan ("[]" = validated-clean, distinct from
// "never ran" — recordPlanRejections' stated contract), and (c) not destroy the
// upfront plan's already-written record. (c) is the overwrite bug: today
// recordPlanRejections rewrites advisor-rejections.json unconditionally, so a
// second call site added naively erases the seeded initial record.
func TestReplan_KnownPhasesOnly_NoSpuriousRejection(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	// Seed the upfront plan's record in the exact shape recordPlanRejections
	// writes today (a flat PlanRejection array).
	seeded, err := json.MarshalIndent([]router.PlanRejection{{
		Phase: seededInitialPhase, Reason: "unknown-phase", Detail: "recorded by the upfront plan",
	}}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "advisor-rejections.json"), seeded, 0o644); err != nil {
		t.Fatal(err)
	}

	pl := &seqReplanPlanner{plans: []*router.PhasePlan{knownPhasePlan()}}
	cr := replanRejCycleRun(t, ws, replanRejOrchestrator(t, pl, 1), 12)

	cr.postScoutReplan()

	if pl.calls != 1 {
		t.Fatalf("RePlan called %d times, want 1", pl.calls)
	}
	rejs := collectWorkspaceRejections(t, ws)
	for _, r := range rejs {
		if strings.Contains(r.Kind, "replan") && r.Reason == "unknown-phase" {
			t.Errorf("all-known re-plan produced a spurious unknown-phase rejection: %+v", r)
		}
	}
	if !replanRecordPresent(t, ws) {
		t.Errorf("a clean re-plan must still record a validated-clean re-plan entry (proof ValidatePlan ran); workspace had no replan-attributed record")
	}
	if !hasRejection(rejs, seededInitialPhase, "unknown-phase", false) {
		t.Errorf("the upfront plan's rejection record was destroyed by the re-plan's record; recovered=%+v", rejs)
	}
}

// TestReplan_MultipleReplans_AllRejectionsRecorded — AC3 (accumulation under
// depth > 1). RePlanMaxDepth allows more than one re-plan per cycle; each must
// keep its own record. An "upfront + latest re-plan" fix still loses the
// intermediate one and fails here.
func TestReplan_MultipleReplans_AllRejectionsRecorded(t *testing.T) {
	t.Parallel()
	ws := scoutWorkspace(t)
	pl := &seqReplanPlanner{plans: []*router.PhasePlan{
		planWithPhantom(phantomReplanPhase),
		planWithPhantom(phantomReplanPhase2),
	}}
	cr := replanRejCycleRun(t, ws, replanRejOrchestrator(t, pl, 2), 13)

	cr.postScoutReplan() // depth 0 → 1
	cr.postScoutReplan() // depth 1 → 2 (cap is 2, so this one re-plans too)

	if pl.calls != 2 {
		t.Fatalf("RePlan called %d times, want 2 (depth cap 2)", pl.calls)
	}
	rejs := collectWorkspaceRejections(t, ws)
	for _, phantom := range []string{phantomReplanPhase, phantomReplanPhase2} {
		if !hasRejection(rejs, phantom, "unknown-phase", true) {
			t.Errorf("rejection for %q is not recoverable — a later re-plan's record overwrote an earlier one; recovered=%+v", phantom, rejs)
		}
	}
}
