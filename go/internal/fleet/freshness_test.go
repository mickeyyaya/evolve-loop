package fleet

// freshness_test.go — TDD contract for the dispatch freshness gate (cycle 767,
// inbox id dispatch-freshness-gate, weight 0.95, campaign loop-reliability-2026-07).
//
// Width-3 batch 2026-07-13 postmortem: ~3 of 8 failed lane-slots were doomed
// at dispatch. (1) push-ci-watch-remote-parity was dispatched AFTER the task
// had shipped (cycle 748) and the honest "no in-scope work remains" build was
// FAILed by the review gate; (2) token-resolver-production-wiring was
// re-picked at 754 after landing at 745 (consumption raced dispatch);
// (3) token-telemetry-s6-rollups was dispatched 3x against an unmet dep.
//
// The gate: immediately before lane launch, re-resolve every spec's scope ids
// against CURRENT inbox/consumed state and deps. Stale ids are skipped with a
// logged reason, the freed slot is refilled from the pending backlog, and an
// honest empty-scope build after the gate verdicts SKIPPED — never FAIL.
//
// Contract the Builder implements (new file freshness.go, package fleet —
// DO NOT modify these tests; make them pass):
//
//	// TaskFreshness is one task id re-resolved at dispatch time.
//	type TaskFreshness struct {
//		Fresh  bool   // still pending in the inbox AND all deps satisfied
//		Reason string // non-empty when !Fresh, e.g. "consumed: promoted processed cycle-748" or "deps unmet: needs <dep-id>"
//	}
//
//	// FreshnessProbeFn re-resolves one task id against current state
//	// (production wiring reads .evolve/inbox lifecycle dirs + deps;
//	// tests inject a fake).
//	type FreshnessProbeFn func(taskID string) TaskFreshness
//
//	// RefillFn returns the next pending backlog item as a lane spec.
//	// exclude holds every id this wave already owns (kept AND skipped) so a
//	// refill can never duplicate a live lane or resurrect a skipped id.
//	// ok=false → no pending candidate; the slot stays empty (a shorter wave,
//	// never a doomed lane).
//	type RefillFn func(exclude map[string]bool) (CycleSpec, bool)
//
//	// FreshnessSkip records one id skipped at dispatch, with its reason.
//	type FreshnessSkip struct {
//		TaskID string
//		Reason string
//	}
//
//	// FreshenSpecs applies the gate: probes every scope id, filters stale
//	// ids out of their specs (a spec whose WHOLE scope went stale is dropped
//	// and its slot refilled; a spec with remaining live ids keeps its slot),
//	// and logs one WARN line per skipped id (id + reason) to warn.
//	// Returns the launchable specs and the skip records.
//	func FreshenSpecs(specs []CycleSpec, probe FreshnessProbeFn, refill RefillFn, warn io.Writer) (kept []CycleSpec, skipped []FreshnessSkip)
//
//	// ClassifyEmptyScopeBuild maps a lane build outcome to its final verdict.
//	// After the freshness gate ran for the lane, an honest
//	// "no in-scope work remains" report is SKIPPED — never FAIL (never punish
//	// an honest empty result), and never PASS either (no work is not work).
//	// Without the gate, or when the build claimed real in-scope work, the
//	// original verdict stands unchanged.
//	func ClassifyEmptyScopeBuild(freshnessGateRan, reportsNoInScopeWork bool, originalVerdict string) string

import (
	"bytes"
	"strings"
	"testing"
)

// freshScopeIDs flattens kept specs to the multiset of ids they own.
func freshScopeIDs(specs []CycleSpec) []string {
	var ids []string
	for _, s := range specs {
		ids = append(ids, s.Scope...)
	}
	return ids
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// AC1 — postmortem shapes (1)/(2): a task consumed/shipped between planning
// and launch is skipped with a logged reason and its slot is REFILLED from
// the pending backlog instead of burning a lane on known-dead work.
func TestWaveDispatch_SkipsConsumedTaskAndRefillsSlot(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-consumed"}},
		{Scope: []string{"task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-consumed" {
			return TaskFreshness{Fresh: false, Reason: "consumed: promoted processed cycle-748"}
		}
		return TaskFreshness{Fresh: true}
	}
	refillCalls := 0
	var gotExclude map[string]bool
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		gotExclude = exclude
		return CycleSpec{Scope: []string{"task-refill"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	ids := freshScopeIDs(kept)
	if containsID(ids, "task-consumed") {
		t.Errorf("consumed task must not be dispatched: kept scope ids = %v", ids)
	}
	if !containsID(ids, "task-live") {
		t.Errorf("fresh task must survive the gate: kept scope ids = %v", ids)
	}
	if !containsID(ids, "task-refill") {
		t.Errorf("freed slot must be refilled from the backlog: kept scope ids = %v", ids)
	}
	if len(kept) != 2 {
		t.Errorf("wave width must be preserved by the refill: got %d specs, want 2", len(kept))
	}
	if refillCalls != 1 {
		t.Errorf("exactly one freed slot → exactly one refill call, got %d", refillCalls)
	}
	if gotExclude == nil || !gotExclude["task-consumed"] || !gotExclude["task-live"] {
		t.Errorf("refill exclude must contain every id the wave already owns (skipped AND kept): got %v", gotExclude)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-consumed" {
		t.Fatalf("want exactly one skip record for task-consumed, got %+v", skipped)
	}
	if !strings.Contains(skipped[0].Reason, "consumed") {
		t.Errorf("skip record must carry the probe's reason, got %q", skipped[0].Reason)
	}
	log := warn.String()
	if !strings.Contains(log, "task-consumed") || !strings.Contains(log, "consumed") {
		t.Errorf("skip must be logged with id + reason, got log:\n%s", log)
	}
}

// AC2 — postmortem shape (3): a task whose declared dependency is still
// unmet at launch is skipped with a reason NAMING the blocking dep, and an
// empty backlog leaves the slot unfilled (a shorter wave, never a doomed lane).
func TestWaveDispatch_SkipsDepsUnmetTaskWithReason(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-blocked"}},
		{Scope: []string{"task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-blocked" {
			return TaskFreshness{Fresh: false, Reason: "deps unmet: needs token-resolver-production-wiring"}
		}
		return TaskFreshness{Fresh: true}
	}
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		return CycleSpec{}, false // backlog exhausted
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	ids := freshScopeIDs(kept)
	if containsID(ids, "task-blocked") {
		t.Errorf("deps-unmet task must not be dispatched: kept scope ids = %v", ids)
	}
	if len(kept) != 1 || !containsID(ids, "task-live") {
		t.Errorf("empty backlog → slot stays empty, fresh lane survives: kept = %v", ids)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-blocked" {
		t.Fatalf("want exactly one skip record for task-blocked, got %+v", skipped)
	}
	if !strings.Contains(skipped[0].Reason, "deps unmet") ||
		!strings.Contains(skipped[0].Reason, "token-resolver-production-wiring") {
		t.Errorf("skip reason must name the unmet dep, got %q", skipped[0].Reason)
	}
	log := warn.String()
	if !strings.Contains(log, "task-blocked") || !strings.Contains(log, "deps unmet") {
		t.Errorf("skip must be logged with id + reason, got log:\n%s", log)
	}
}

// AC3 — the backstop for the review-gate injustice (postmortem shape (1)):
// an HONEST empty-scope build after the freshness gate verdicts SKIPPED —
// never FAIL, and never PASS. Real failures and pre-gate behavior are
// untouched (negative rows: the classifier must not mask genuine outcomes).
func TestBuildEmptyScope_AfterFreshnessGate_VerdictSkippedNotFail(t *testing.T) {
	cases := []struct {
		name             string
		gateRan          bool
		reportsNoInScope bool
		original         string
		want             string
	}{
		{"honest empty scope after gate is SKIPPED not FAIL", true, true, "FAIL", "SKIPPED"},
		{"honest empty scope after gate never counts as PASS", true, true, "PASS", "SKIPPED"},
		{"no gate ran: original FAIL preserved (pre-gate behavior)", false, true, "FAIL", "FAIL"},
		{"real in-scope FAIL is never masked", true, false, "FAIL", "FAIL"},
		{"real in-scope PASS untouched", true, false, "PASS", "PASS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEmptyScopeBuild(tc.gateRan, tc.reportsNoInScope, tc.original)
			if got != tc.want {
				t.Errorf("ClassifyEmptyScopeBuild(gateRan=%v, empty=%v, %q) = %q, want %q",
					tc.gateRan, tc.reportsNoInScope, tc.original, got, tc.want)
			}
		})
	}
}

// Negative (anti-no-op): an all-fresh wave passes through the gate untouched —
// no skips, no refills, no log noise, order preserved. A gate that "fixes"
// waves by rewriting healthy ones would pass AC1/AC2 and fail here.
func TestWaveDispatch_AllFresh_NoSkipNoRefill(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-a"}},
		{Scope: []string{"task-b", "task-c"}},
	}
	probe := func(id string) TaskFreshness { return TaskFreshness{Fresh: true} }
	refillCalls := 0
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		return CycleSpec{Scope: []string{"task-never"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	if len(skipped) != 0 {
		t.Errorf("all-fresh wave must skip nothing, got %+v", skipped)
	}
	if refillCalls != 0 {
		t.Errorf("all-fresh wave must never call refill, got %d calls", refillCalls)
	}
	if warn.Len() != 0 {
		t.Errorf("all-fresh wave must log nothing, got:\n%s", warn.String())
	}
	if len(kept) != 2 ||
		strings.Join(kept[0].Scope, ",") != "task-a" ||
		strings.Join(kept[1].Scope, ",") != "task-b,task-c" {
		t.Errorf("all-fresh specs must pass through unchanged in order, got %+v", kept)
	}
}

// Edge — postmortem shape (2) at merged-spec granularity: a spec that merged
// two file-sharing todos where ONE was consumed keeps its slot with the scope
// FILTERED to the live id (the lane still has real work; no refill fires).
func TestWaveDispatch_PartialStaleScope_FiltersIdKeepsSpec(t *testing.T) {
	specs := []CycleSpec{
		{Scope: []string{"task-consumed", "task-live"}},
	}
	probe := func(id string) TaskFreshness {
		if id == "task-consumed" {
			return TaskFreshness{Fresh: false, Reason: "consumed: promoted processed cycle-745"}
		}
		return TaskFreshness{Fresh: true}
	}
	refillCalls := 0
	refill := func(exclude map[string]bool) (CycleSpec, bool) {
		refillCalls++
		return CycleSpec{Scope: []string{"task-never"}}, true
	}
	var warn bytes.Buffer

	kept, skipped := FreshenSpecs(specs, probe, refill, &warn)

	if len(kept) != 1 || strings.Join(kept[0].Scope, ",") != "task-live" {
		t.Errorf("partially-stale spec must keep its slot with scope filtered to live ids, got %+v", kept)
	}
	if refillCalls != 0 {
		t.Errorf("a slot that still has live work must not be refilled, got %d refill calls", refillCalls)
	}
	if len(skipped) != 1 || skipped[0].TaskID != "task-consumed" {
		t.Errorf("the consumed id must be recorded as skipped, got %+v", skipped)
	}
}
