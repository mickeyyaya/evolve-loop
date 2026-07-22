//go:build acs

// Package cycle1062 materialises the cycle-1062 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	chronicle-s6-escalation-boundary
//	  → superseded_by: failure-disposition-router (S4 boundary applier)
//
// Because the parent design states "S4 MUST NOT land before S3's staging
// exists", scout materialised the committed task as two dependency-ordered
// halves, both of which these predicates gate:
//
//	Task 1  disposition-router-s3-floors-and-staging   → go/internal/dispositionrouter
//	Task 2  disposition-router-s4-boundary-applier     → go/internal/recurrence/apply.go
//	                                                     + go/cmd/evolve/cmd_loop.go call site
//
// Predicate strategy — every predicate here EXERCISES the system under test
// (calls the production function against a t.TempDir() fixture and asserts on
// its return value or its real on-disk side effect), never a source-grep of
// production code (the cycle-85 degenerate-predicate ban). Predicates 001-004
// drive `dispositionrouter` directly; 005-009 drive `recurrence.ApplyBoundary` directly;
// 010 shells the loop-boundary wiring test so the cmd_loop call site is proven
// by execution, not by a magic string.
//
// RED shape this cycle: the `dispositionrouter` package and `recurrence.ApplyBoundary` do
// not exist yet, so the package fails to COMPILE — the correct RED for a
// not-yet-built API (go/acs/README.md: a predicate package that fails to
// compile is a hard suite error, never a silent PASS).
//
// Isolation: no predicate reads or writes the live repo tree. Every inbox,
// escalations-staging and report path is rooted in t.TempDir(), per the cycle
// goal constraint that tests must never mutate the live tree.
package cycle1062

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// writeInboxItem writes an OPEN inbox item carrying id/pattern/weight at the
// top level of inboxDir (the dispatchable location — a CLAIMED item lives under
// inboxDir/processing/cycle-<N>/ instead, per inboxmover.Claim).
func writeInboxItem(t *testing.T, inboxDir, id, pattern string, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	path := filepath.Join(inboxDir, id+".json")
	body, err := json.MarshalIndent(map[string]any{
		"id":      id,
		"action":  "fix " + pattern,
		"weight":  weight,
		"pattern": pattern,
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode inbox item: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write inbox item: %v", err)
	}
	return path
}

// claimInboxItem moves an open item into inboxDir/processing/cycle-<cycle>/,
// reproducing inboxmover.Claim's os.Rename exactly — the race the chronicle-s6
// spec pins: an applier that re-files or bumps a CLAIMED item resurrects it into
// double work across fleet lanes.
func claimInboxItem(t *testing.T, inboxDir, openPath, cycle string) string {
	t.Helper()
	destDir := filepath.Join(inboxDir, "processing", "cycle-"+cycle)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir processing: %v", err)
	}
	dest := filepath.Join(destDir, filepath.Base(openPath))
	if err := os.Rename(openPath, dest); err != nil {
		t.Fatalf("claim rename: %v", err)
	}
	return dest
}

// stageEscalate stages one "escalate" intent (bump an existing open item) via
// the PRODUCTION staging writer, so the applier consumes exactly what S3 emits.
func stageEscalate(t *testing.T, escDir string, cycle int, pattern, itemID string, count int, weight float64) {
	t.Helper()
	if _, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      cycle,
		Pattern:    pattern,
		ItemID:     itemID,
		Action:     "escalate",
		Route:      "queue",
		Recurrence: count,
		Weight:     weight,
	}); err != nil {
		t.Fatalf("StageIntent(escalate): %v", err)
	}
}

// stageAutofile stages one "autofile" intent (no open item exists for the
// pattern, so the recurrence must land on the queue as a NEW item).
func stageAutofile(t *testing.T, escDir string, cycle int, pattern, itemID string, count int, weight float64) {
	t.Helper()
	if _, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      cycle,
		Pattern:    pattern,
		ItemID:     itemID,
		Action:     "autofile",
		Route:      "queue",
		Recurrence: count,
		Weight:     weight,
	}); err != nil {
		t.Fatalf("StageIntent(autofile): %v", err)
	}
}

// applyOpts builds the boundary-applier options for a temp fixture root.
func applyOpts(root string, cycle int, shadow bool) recurrence.ApplyOptions {
	return recurrence.ApplyOptions{
		InboxDir:        filepath.Join(root, "inbox"),
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(root, "escalations")),
		ReportPath:      filepath.Join(root, "escalation-apply-report.json"),
		Cycle:           cycle,
		Shadow:          shadow,
		Policy:          recurrence.DefaultEscalationPolicy(),
		Now:             time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
}

// itemWeight reads the "weight" field of the inbox item whose "id" == id,
// searching the whole inbox tree (open items and claimed items alike).
// ok=false when no item with that id exists anywhere.
func itemWeight(t *testing.T, inboxDir, id string) (weight float64, path string, ok bool) {
	t.Helper()
	_ = filepath.Walk(inboxDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".json") || ok {
			return nil //nolint:nilerr // absent tree is a legitimate "not found"
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var it struct {
			ID     string  `json:"id"`
			Weight float64 `json:"weight"`
		}
		if json.Unmarshal(raw, &it) != nil || it.ID != id {
			return nil
		}
		weight, path, ok = it.Weight, p, true
		return nil
	})
	return weight, path, ok
}

// openItemCount counts dispatchable (top-level) inbox items — claimed items
// under processing/ are excluded, matching what a fleet lane can draw.
func openItemCount(t *testing.T, inboxDir string) int {
	t.Helper()
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// S3 — router floors + staging (Task 1)
// ---------------------------------------------------------------------------

// TestC1062_001_RouterFloorForcesConsoleForGuardAbort — AC: the deterministic
// floor forces console routing for the guard-abort pre-class (a severed
// statemap is operator-owned; a lane must never draw it). Behavioural: calls
// dispositionrouter.Decide and asserts the returned Route/Forced, so a stub that returns a
// zero Decision fails.
func TestC1062_001_RouterFloorForcesConsoleForGuardAbort(t *testing.T) {
	d := dispositionrouter.Decide("guard-abort", 1, "queue")
	if d.Route != "console" {
		t.Errorf("Decide(guard-abort, 1, queue).Route = %q, want \"console\" (floor)", d.Route)
	}
	if !d.Forced {
		t.Errorf("Decide(guard-abort, 1, queue).Forced = false, want true (floor decisions are forced)")
	}
	if d.Reason == "" {
		t.Errorf("forced Decision carries an empty Reason; the floor must say why")
	}

	// Negative / semantic contrast: an ordinary class at recurrence 1 must NOT
	// be force-consoled, else the floor is a blanket console-everything no-op.
	if q := dispositionrouter.Decide("verdict-fail", 1, "queue"); q.Route != "queue" || q.Forced {
		t.Errorf("Decide(verdict-fail, 1, queue) = {Route:%q Forced:%v}, want {queue false}", q.Route, q.Forced)
	}
}

// TestC1062_002_RouterFloorForcesConsoleAtRecurrenceThree — AC: recurrence >= 3
// forces console regardless of pre-class (a defect that survived two fixes is
// no longer a lane-sized task). Boundary-exact: 2 must NOT force, 3 must.
func TestC1062_002_RouterFloorForcesConsoleAtRecurrenceThree(t *testing.T) {
	if d := dispositionrouter.Decide("verdict-fail", 3, "queue"); d.Route != "console" || !d.Forced {
		t.Errorf("Decide(verdict-fail, 3, queue) = {Route:%q Forced:%v}, want {console true}", d.Route, d.Forced)
	}
	if d := dispositionrouter.Decide("verdict-fail", 9, "queue"); d.Route != "console" || !d.Forced {
		t.Errorf("Decide(verdict-fail, 9, queue) = {Route:%q Forced:%v}, want {console true}", d.Route, d.Forced)
	}
	// Edge: one below the floor must stay on the queue.
	if d := dispositionrouter.Decide("verdict-fail", 2, "queue"); d.Route != "queue" || d.Forced {
		t.Errorf("Decide(verdict-fail, 2, queue) = {Route:%q Forced:%v}, want {queue false} (floor is >=3)", d.Route, d.Forced)
	}
}

// TestC1062_003_RouterLLMMayRaiseNeverLowerForcedRouting — AC: the advisory LLM
// route may RAISE queue→console, but may never LOWER a floor-forced console to
// queue. This is the anti-gaming pin: an implementation that simply trusts the
// llmRoute argument fails the lowering case.
func TestC1062_003_RouterLLMMayRaiseNeverLowerForcedRouting(t *testing.T) {
	// Raise: no floor fires, advisory says console → console (not forced).
	raised := dispositionrouter.Decide("verdict-fail", 1, "console")
	if raised.Route != "console" {
		t.Errorf("advisory raise ignored: Decide(verdict-fail, 1, console).Route = %q, want \"console\"", raised.Route)
	}

	// Lower (negative test): floor forced console, advisory says queue → console.
	for _, tc := range []struct {
		preClass   string
		recurrence int
	}{
		{"guard-abort", 1},
		{"verdict-fail", 3},
	} {
		got := dispositionrouter.Decide(tc.preClass, tc.recurrence, "queue")
		if got.Route != "console" || !got.Forced {
			t.Errorf("advisory LOWERED a forced route: Decide(%s, %d, queue) = {Route:%q Forced:%v}, want {console true}",
				tc.preClass, tc.recurrence, got.Route, got.Forced)
		}
	}

	// Edge: an empty/unknown advisory route must not corrupt the outcome.
	if d := dispositionrouter.Decide("verdict-fail", 1, ""); d.Route != "queue" {
		t.Errorf("Decide(verdict-fail, 1, \"\").Route = %q, want \"queue\" (empty advisory is a no-op)", d.Route)
	}
}

// TestC1062_004_StagedIntentNeverWritesInboxMidFlight — AC (race-proven,
// carried verbatim from chronicle-s6): S3 stages intents to
// .evolve/escalations/pending-actions.jsonl and MUST NOT write .evolve/inbox/
// mid-flight, where a write would race inboxmover.Claim's os.Rename and
// resurrect a claimed item into double work. Behavioural: calls StageIntent
// against a temp root and asserts (a) the JSONL record round-trips and (b) the
// inbox tree is byte-identical afterwards.
func TestC1062_004_StagedIntentNeverWritesInboxMidFlight(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	openPath := writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	before, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("read seeded item: %v", err)
	}
	beforeCount := openItemCount(t, inboxDir)

	staged, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      1062,
		Pattern:    "pattern:flaky-tier",
		ItemID:     "recurring-defect",
		Action:     "escalate",
		Route:      "queue",
		Recurrence: 4,
		Weight:     0.80,
	})
	if err != nil {
		t.Fatalf("StageIntent: %v (S3 must create .evolve/escalations/ on first run)", err)
	}
	if want := dispositionrouter.PendingActionsPath(escDir); staged != want {
		t.Errorf("StageIntent path = %q, want %q", staged, want)
	}

	// (a) the staged record is real, parseable JSONL carrying the intent.
	raw, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("staged file not readable: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("staged file has %d JSONL lines, want 1", len(lines))
	}
	var got dispositionrouter.Intent
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("staged line is not valid JSON: %v", err)
	}
	if got.ItemID != "recurring-defect" || got.Action != "escalate" || got.Recurrence != 4 {
		t.Errorf("staged intent round-trip = %+v, want ItemID=recurring-defect Action=escalate Recurrence=4", got)
	}

	// (b) the inbox is untouched — the race pin.
	after, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("seeded inbox item disappeared during staging: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("StageIntent MUTATED an inbox item (race with inboxmover.Claim); staging must never write the inbox")
	}
	if n := openItemCount(t, inboxDir); n != beforeCount {
		t.Errorf("open inbox item count %d → %d during staging; want unchanged", beforeCount, n)
	}
}

// ---------------------------------------------------------------------------
// S4 — boundary applier (Task 2, subsumes chronicle-s6-escalation-boundary)
// ---------------------------------------------------------------------------

// TestC1062_005_ApplyEscalationIdempotentPerCycleStamp — AC: applying the same
// staged intent twice in the same cycle escalates EXACTLY once (per-cycle
// idempotency stamp). Behavioural: runs ApplyBoundary twice and asserts the
// second run bumps nothing and leaves the weight where the first run left it.
func TestC1062_005_ApplyEscalationIdempotentPerCycleStamp(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	stageEscalate(t, escDir, 1062, "pattern:flaky-tier", "recurring-defect", 4, 0.80)

	first, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (first): %v", err)
	}
	if len(first.Bumped) != 1 {
		t.Fatalf("first apply Bumped = %v, want exactly 1 item", first.Bumped)
	}
	afterFirst, _, ok := itemWeight(t, inboxDir, "recurring-defect")
	if !ok {
		t.Fatalf("item vanished after first apply")
	}
	if want := recurrence.DefaultEscalationPolicy().Target(0.80, 4); afterFirst != want {
		t.Errorf("weight after first apply = %v, want %v (min(cap, base+step*(count-1)))", afterFirst, want)
	}

	second, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Bumped) != 0 {
		t.Errorf("second apply in the SAME cycle bumped %v, want none (per-cycle stamp)", second.Bumped)
	}
	afterSecond, _, _ := itemWeight(t, inboxDir, "recurring-defect")
	if afterSecond != afterFirst {
		t.Errorf("weight drifted on re-apply: %v → %v, want stable", afterFirst, afterSecond)
	}
}

// TestC1062_006_ApplyEscalationNeverLowersWeight — AC (negative test): an
// intent whose computed target is BELOW the item's current weight must leave
// the weight alone. An applier that blindly assigns the computed target
// silently demotes an already-hot item; this is the strongest anti-no-op pin
// on the escalation math.
func TestC1062_006_ApplyEscalationNeverLowersWeight(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	const hot = 0.97
	writeInboxItem(t, inboxDir, "already-hot", "pattern:hot", hot)
	// Recurrence 2 from a stale base of 0.50 computes ~0.53 — far below 0.97.
	stageEscalate(t, escDir, 1062, "pattern:hot", "already-hot", 2, 0.50)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	got, _, ok := itemWeight(t, inboxDir, "already-hot")
	if !ok {
		t.Fatalf("item vanished during apply")
	}
	if got < hot {
		t.Errorf("apply LOWERED weight %v → %v; escalation must never lower (result: %+v)", hot, got, res)
	}
}

// TestC1062_007_PlanEscalationSkipsClaimedItems — AC (race-proven, verbatim
// from chronicle-s6): an item already CLAIMED by a fleet lane (moved under
// inbox/processing/cycle-N/ by inboxmover.Claim) must be skipped, never bumped
// and never re-filed at the top level — re-filing resurrects it into double
// work across lanes.
func TestC1062_007_PlanEscalationSkipsClaimedItems(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	openPath := writeInboxItem(t, inboxDir, "claimed-item", "pattern:claimed", 0.70)
	claimed := claimInboxItem(t, inboxDir, openPath, "1061")
	stageEscalate(t, escDir, 1062, "pattern:claimed", "claimed-item", 5, 0.70)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	for _, id := range res.Bumped {
		if id == "claimed-item" {
			t.Errorf("applier bumped a CLAIMED item (%s); claimed items are in flight and must be skipped", id)
		}
	}
	if n := openItemCount(t, inboxDir); n != 0 {
		t.Errorf("applier RESURRECTED %d claimed item(s) into the dispatchable inbox; want 0", n)
	}
	got, path, ok := itemWeight(t, inboxDir, "claimed-item")
	if !ok {
		t.Fatalf("claimed item disappeared entirely (was at %s)", claimed)
	}
	if got != 0.70 {
		t.Errorf("claimed item weight mutated 0.70 → %v (at %s); want untouched", got, path)
	}
	if len(res.Skipped) == 0 {
		t.Errorf("result.Skipped is empty; a skipped claimed item must be reported, not silently dropped")
	}
}

// TestC1062_008_ShadowStageWritesReportOnly — AC: shadow stage (the compiled
// default for this design) writes the escalation report artifact and performs
// ZERO inbox mutation. Behavioural: runs ApplyBoundary with Shadow=true and
// asserts the report exists AND both the escalate and autofile paths left the
// inbox exactly as seeded.
func TestC1062_008_ShadowStageWritesReportOnly(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	stageEscalate(t, escDir, 1062, "pattern:flaky-tier", "recurring-defect", 4, 0.80)
	stageAutofile(t, escDir, 1062, "pattern:orphan", "orphan-defect", 3, 0.85)
	before := openItemCount(t, inboxDir)

	opts := applyOpts(root, 1062, true)
	res, err := recurrence.ApplyBoundary(opts)
	if err != nil {
		t.Fatalf("ApplyBoundary(shadow): %v", err)
	}
	if !res.Shadow {
		t.Errorf("result.Shadow = false for a shadow run; the report must record the stage")
	}
	if !acsassert.FileExists(t, opts.ReportPath) {
		t.Fatalf("shadow run wrote no report at %s; shadow must still emit the artifact", opts.ReportPath)
	}
	var report struct {
		Cycle  int  `json:"cycle"`
		Shadow bool `json:"shadow"`
	}
	raw, err := os.ReadFile(opts.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if report.Cycle != 1062 || !report.Shadow {
		t.Errorf("report = {cycle:%d shadow:%v}, want {1062 true}", report.Cycle, report.Shadow)
	}

	// Zero inbox mutation: no new item filed, no weight moved.
	if n := openItemCount(t, inboxDir); n != before {
		t.Errorf("shadow run changed open inbox item count %d → %d; shadow must not mutate the inbox", before, n)
	}
	if w, _, _ := itemWeight(t, inboxDir, "recurring-defect"); w != 0.80 {
		t.Errorf("shadow run bumped a weight 0.80 → %v; shadow is report-only", w)
	}
	if _, _, ok := itemWeight(t, inboxDir, "orphan-defect"); ok {
		t.Errorf("shadow run FILED the autofile intent into the inbox; shadow is report-only")
	}
}

// TestC1062_009_AutofileGoesThroughRetrofile — AC: the autofile path is wired
// through the existing (until now caller-less) retrofile package rather than a
// hand-rolled second filer — retrofile gains its first production caller.
// Behavioural: an enforce-stage apply of an autofile intent must produce a real
// inbox item bearing retrofile's own emitted shape (auto-retro-<cycle>-<id>.json
// with injected_by=retro-preventive-actions-autofiler), which only
// retrofile.FileActions writes.
func TestC1062_009_AutofileGoesThroughRetrofile(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	stageAutofile(t, escDir, 1062, "pattern:orphan", "orphan-defect", 3, 0.85)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Filed) != 1 {
		t.Fatalf("Filed = %v, want exactly 1 autofiled item", res.Filed)
	}

	wantPath := filepath.Join(inboxDir, "auto-retro-1062-orphan-defect.json")
	if !acsassert.FileExists(t, wantPath) {
		t.Fatalf("autofile did not emit %s; the autofile backend must be retrofile.FileActions, not a hand-rolled filer", wantPath)
	}
	raw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read filed item: %v", err)
	}
	var item struct {
		ID         string  `json:"id"`
		Weight     float64 `json:"weight"`
		Recurrence int     `json:"recurrence"`
		InjectedBy string  `json:"injected_by"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("filed item is not valid JSON: %v", err)
	}
	if item.ID != "orphan-defect" {
		t.Errorf("filed item id = %q, want \"orphan-defect\"", item.ID)
	}
	if item.InjectedBy != "retro-preventive-actions-autofiler" {
		t.Errorf("filed item injected_by = %q, want retrofile's own value \"retro-preventive-actions-autofiler\"", item.InjectedBy)
	}
	if item.Weight != 0.85 {
		t.Errorf("filed item weight = %v, want the staged 0.85 (WeightHint must reach retrofile)", item.Weight)
	}
	if item.Recurrence != 3 {
		t.Errorf("filed item recurrence = %d, want 3 (the recurrence count must reach the queue)", item.Recurrence)
	}

	// Idempotency across the same cycle: re-applying must not double-file
	// (retrofile's own dedup, exercised through the boundary applier).
	again, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (re-apply): %v", err)
	}
	if len(again.Filed) != 0 {
		t.Errorf("re-apply filed %v again; autofile must happen exactly once while the item is open", again.Filed)
	}
}

// TestC1062_010_LoopBoundaryWiringExecutes — AC: the applier is actually CALLED
// at the cmd_loop per-iteration boundary (after dispatchIteration returns with
// no lanes in flight), not merely defined. Executing the loop-boundary test is
// the wiring proof — a source-grep for the call site would pass on a commented
// reference, so this predicate shells the real test binary instead.
//
// Guards against the empty-selector trap: `go test -run <missing>` exits 0 with
// "no tests to run", so the predicate asserts the named test actually RAN.
func TestC1062_010_LoopBoundaryWiringExecutes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "cmd", "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestRunLoop_EscalatesAtIterationBoundary", "-v", pkgDir,
	)
	combined := stdout + stderr
	if err != nil && code == 0 {
		t.Fatalf("could not run the loop-boundary wiring test: %v", err)
	}
	if code != 0 {
		t.Fatalf("loop-boundary wiring test FAILED (exit %d):\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestRunLoop_EscalatesAtIterationBoundary") {
		t.Fatalf("TestRunLoop_EscalatesAtIterationBoundary did not run (no tests matched) — the cmd_loop boundary call site is unwired:\n%s", combined)
	}
	if !strings.Contains(combined, "--- PASS: TestRunLoop_EscalatesAtIterationBoundary") {
		t.Errorf("expected an explicit PASS line for TestRunLoop_EscalatesAtIterationBoundary:\n%s", combined)
	}
}
