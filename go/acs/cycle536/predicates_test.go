//go:build acs

// Package cycle536 materialises the cycle-536 acceptance criteria for the THREE
// triage-committed (`## top_n`) tasks.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly THREE tasks to this cycle:
//	  1. carryover-legacy-expiry-backfill      (H) — C536_001..004
//	  2. carryover-cycles-unpicked-increment   (M) — C536_005..006
//	  3. wire-fleet-width-topn-selection       (H) — C536_007..008
//	plus one shared no-regression hygiene predicate — C536_009.
//	Every `## deferred` item (5 stamped legacy todos, the 6 token-optimization
//	inbox items, next-cycle-brief staleness) gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//   - Task 1: `state.json:carryoverTodos` grew to 76 entries; 71 predate the
//     TTL-stamping fix and carry NO `expiresAt`, so PruneExpiredCarryoverTodos
//     keeps them forever (age unknown → never delete). They re-render into every
//     advisor/router prompt and are on track to force a manual array-wipe (the
//     cycle-360 incident). A ONE-TIME backfill stamps a conservative default TTL
//     on every legacy row so the existing prune path can converge them.
//   - Task 2: `CyclesUnpicked` is a dead field — every write site hardcodes 0 and
//     nothing increments it, yet the advisor prompt renders it as a staleness
//     signal. A per-boot increment for every SURVIVING carryover todo makes the
//     field mean what its name promises.
//   - Task 3: `triagecap.SelectFleetWidthTopN` (fleet-width-aware, file-disjoint
//     top_n packing) ships fully tested but has ZERO production callers across two
//     prior processing attempts (cycles 508, 518). This cycle wires it into the
//     wave-seed path so `evolve cycle run` supplies >= fleet.count mutually
//     file-disjoint lanes instead of a raw weight-sorted top-N (which can hand the
//     fleet two candidates that collide on a shared file).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real production function against a seeded temp state.json / temp
// inbox and asserts on the returned value AND the on-disk side effect, never a
// "source file contains text X" grep. C536_009 runs `go build`/`go vet` as real
// subprocesses.
//
// The suite is RED on the current tree by COMPILE FAILURE: the three Builder
// symbols below are all undefined —
//   - failurelog.BackfillLegacyCarryoverExpiry(statePath string, defaultTTL time.Duration, now time.Time) (stamped int, err error)
//   - failurelog.IncrementCarryoverUnpicked(statePath string) (incremented int, err error)
//   - triagecap.SelectWaveSeedTopN(evolveDir string, count int) []triagecap.FleetCandidate
//
// (see test-report.md handoff for the full Builder contract).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C536_001 stamps legacy + leaves already-stamped untouched.
//   - Idempotent: C536_002 a second backfill stamps zero more (anti-double-stamp).
//   - End-to-end: C536_003 backfilled entry is actually PRUNED once past its TTL.
//   - Edge     : C536_004 / C536_006 missing & empty state are safe no-ops.
//   - Positive : C536_005 increments 0,2,5 -> 1,3,6 by EXACTLY one (anti-reset).
//   - Negative : C536_007 the disjoint pick REJECTS the raw top-weight pair that
//     shares a file — a weight-only implementation FAILS here (the anti-no-op).
//   - Edge     : C536_008 count<2 reproduces the legacy single-focus pick.
//   - Hygiene  : C536_009 touched packages build + vet clean (no regression).
package cycle536

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// defaultTTL is the conservative 30-day backfill TTL the scout selected (matches
// the existing CodeBuildFail/CodeAuditFail taxonomy — no new classification).
const defaultTTL = 30 * 24 * time.Hour

// writeState writes body to <dir>/state.json and returns the path.
func writeState(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// readTodos returns the carryoverTodos array parsed from state.json on disk.
func readTodos(t *testing.T, statePath string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json is not valid JSON after the pass: %v", err)
	}
	entries, _ := state["carryoverTodos"].([]any)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// findTodo returns the carryover todo with the given id (or fails).
func findTodo(t *testing.T, todos []map[string]any, id string) map[string]any {
	t.Helper()
	for _, m := range todos {
		if s, _ := m["id"].(string); s == id {
			return m
		}
	}
	t.Fatalf("carryover todo %q not found on disk", id)
	return nil
}

// ---------------------------------------------------------------------------
// Task 1 — carryover-legacy-expiry-backfill
// ---------------------------------------------------------------------------

// TestC536_001_BackfillStampsLegacyLeavesStampedUntouched is the core positive:
// a legacy entry (no expiresAt) is stamped exactly now+TTL, while an entry that
// ALREADY carries an expiresAt is left byte-identical. Exercises the real
// BackfillLegacyCarryoverExpiry against a seeded state.json and asserts BOTH the
// return count and the on-disk mutation.
func TestC536_001_BackfillStampsLegacyLeavesStampedUntouched(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	preStamped := now.Add(72 * time.Hour).Format(time.RFC3339)
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"cycle-366-legacy-no-ttl","action":"legacy"},`+
		`{"id":"cycle-528-already-stamped","expiresAt":"`+preStamped+`"}]}`)

	stamped, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now)
	if err != nil {
		t.Fatalf("BackfillLegacyCarryoverExpiry: %v", err)
	}
	if stamped != 1 {
		t.Errorf("exactly one legacy entry should be stamped; got stamped=%d", stamped)
	}

	todos := readTodos(t, statePath)

	legacy := findTodo(t, todos, "cycle-366-legacy-no-ttl")
	got, _ := legacy["expiresAt"].(string)
	if got == "" {
		t.Fatalf("legacy entry must gain an expiresAt after backfill; got none")
	}
	parsed, perr := time.Parse(time.RFC3339, got)
	if perr != nil {
		t.Fatalf("backfilled expiresAt must be RFC3339; got %q err=%v", got, perr)
	}
	if want := now.Add(defaultTTL); !parsed.Equal(want) {
		t.Errorf("legacy entry expiresAt = %s, want now+TTL = %s", parsed, want)
	}

	stampedEntry := findTodo(t, todos, "cycle-528-already-stamped")
	if got := stampedEntry["expiresAt"]; got != preStamped {
		t.Errorf("already-stamped entry must be left untouched; expiresAt = %v, want %q", got, preStamped)
	}
}

// TestC536_002_BackfillIsIdempotent pins that a SECOND backfill pass stamps zero
// additional entries — once an entry has an expiresAt the pass skips it. Guards
// against a re-stamp that would push a converging entry's TTL forward every boot
// (which would defeat the whole prune-convergence goal).
func TestC536_002_BackfillIsIdempotent(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"cycle-400-legacy-a","action":"legacy"},`+
		`{"id":"cycle-450-legacy-b","action":"legacy"}]}`)

	first, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if first != 2 {
		t.Fatalf("first pass should stamp both legacy entries; got %d", first)
	}
	firstDisk, _ := os.ReadFile(statePath)

	// A later boot: same TTL, a LATER now. Nothing should change.
	second, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if second != 0 {
		t.Errorf("second pass must stamp zero already-stamped entries (idempotent); got %d", second)
	}
	secondDisk, _ := os.ReadFile(statePath)
	if string(firstDisk) != string(secondDisk) {
		t.Errorf("idempotent backfill must not rewrite already-stamped expiresAt:\nfirst:  %s\nsecond: %s", firstDisk, secondDisk)
	}
}

// TestC536_003_BackfilledEntryPrunesOncePastTTL closes the loop end-to-end: a
// legacy entry is backfilled, then PruneExpiredCarryoverTodos run past the
// stamped TTL actually REMOVES it. This proves the backfill hands the existing
// prune path a real, convergeable expiry (not a cosmetic no-op field).
func TestC536_003_BackfilledEntryPrunesOncePastTTL(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	statePath := writeState(t, `{"carryoverTodos":[{"id":"cycle-372-legacy","action":"legacy"}]}`)

	if _, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, defaultTTL, now); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// Before the TTL elapses the entry must survive a prune...
	if pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, now.Add(defaultTTL-time.Hour)); err != nil || pr.Removed != 0 {
		t.Fatalf("backfilled entry must survive until its TTL; pr=%+v err=%v", pr, err)
	}
	// ...and be removed once past it.
	pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, now.Add(defaultTTL+time.Hour))
	if err != nil {
		t.Fatalf("prune past TTL: %v", err)
	}
	if pr.Removed != 1 || pr.After != 0 {
		t.Errorf("backfilled entry must be pruned once past its TTL; got %+v", pr)
	}
	if len(readTodos(t, statePath)) != 0 {
		t.Error("state.json must have zero carryoverTodos after the entry is pruned")
	}
}

// TestC536_004_BackfillMissingOrEmptyIsSafeNoOp is the edge case: a missing
// state.json and a state with no carryoverTodos are both safe no-ops (zero
// stamped, no error) — the backfill must never abort loop boot.
func TestC536_004_BackfillMissingOrEmptyIsSafeNoOp(t *testing.T) {
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	missing := filepath.Join(t.TempDir(), "nope.json")
	if n, err := failurelog.BackfillLegacyCarryoverExpiry(missing, defaultTTL, now); err != nil || n != 0 {
		t.Fatalf("missing state must be a safe no-op; got stamped=%d err=%v", n, err)
	}

	empty := writeState(t, `{"failedApproaches":[]}`)
	if n, err := failurelog.BackfillLegacyCarryoverExpiry(empty, defaultTTL, now); err != nil || n != 0 {
		t.Fatalf("state with no carryoverTodos must be a no-op; got stamped=%d err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// Task 2 — carryover-cycles-unpicked-increment
// ---------------------------------------------------------------------------

// TestC536_005_IncrementBumpsEverySurvivorByExactlyOne pins the staleness
// counter: three todos at cycles_unpicked 0,2,5 become 1,3,6 after one pass —
// each bumped by EXACTLY one. A reset-to-0 or a double-bump (both plausible
// wrong implementations) fails here.
func TestC536_005_IncrementBumpsEverySurvivorByExactlyOne(t *testing.T) {
	statePath := writeState(t, `{"carryoverTodos":[`+
		`{"id":"a","cycles_unpicked":0},`+
		`{"id":"b","cycles_unpicked":2},`+
		`{"id":"c","cycles_unpicked":5}]}`)

	n, err := failurelog.IncrementCarryoverUnpicked(statePath)
	if err != nil {
		t.Fatalf("IncrementCarryoverUnpicked: %v", err)
	}
	if n != 3 {
		t.Errorf("all three surviving todos should be incremented; got incremented=%d", n)
	}

	want := map[string]float64{"a": 1, "b": 3, "c": 6}
	for _, m := range readTodos(t, statePath) {
		id, _ := m["id"].(string)
		got, ok := m["cycles_unpicked"].(float64) // JSON numbers decode to float64
		if !ok {
			t.Errorf("todo %q lost its cycles_unpicked field", id)
			continue
		}
		if got != want[id] {
			t.Errorf("todo %q cycles_unpicked = %v, want %v (exactly +1)", id, got, want[id])
		}
	}
}

// TestC536_006_IncrementMissingOrEmptyIsSafeNoOp is the edge case: missing
// state.json and a state with no carryoverTodos are safe no-ops — the increment
// must never abort loop boot.
func TestC536_006_IncrementMissingOrEmptyIsSafeNoOp(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.json")
	if n, err := failurelog.IncrementCarryoverUnpicked(missing); err != nil || n != 0 {
		t.Fatalf("missing state must be a safe no-op; got incremented=%d err=%v", n, err)
	}

	empty := writeState(t, `{"failedApproaches":[]}`)
	if n, err := failurelog.IncrementCarryoverUnpicked(empty); err != nil || n != 0 {
		t.Fatalf("state with no carryoverTodos must be a no-op; got incremented=%d err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// Task 3 — wire-fleet-width-topn-selection
// ---------------------------------------------------------------------------

// writeInboxTodo writes one inbox todo JSON into <evolveDir>/inbox/.
func writeInboxTodo(t *testing.T, evolveDir, id string, weight float64, files []string) {
	t.Helper()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"id": id, "weight": weight, "files": files}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, id+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func repIDs(reps []triagecap.FleetCandidate) []string {
	out := make([]string, len(reps))
	for i, r := range reps {
		out[i] = r.ID
	}
	return out
}

// TestC536_007_WaveSeedPicksDisjointRepsNotRawTopWeight is the load-bearing
// negative that proves the wiring. The inbox holds three todos:
//
//	alpha  weight 0.9  files [x.go]
//	beta   weight 0.8  files [x.go]   <- shares x.go with alpha
//	gamma  weight 0.7  files [y.go]
//
// A raw top-2-by-weight selection (the pre-wiring behaviour) returns {alpha,
// beta} — two lanes that COLLIDE on x.go. The wired SelectFleetWidthTopN must
// return {alpha, gamma}: alpha wins its file bucket over beta on weight, gamma
// is the disjoint runner-up. Exercises the real SelectWaveSeedTopN end-to-end
// (inbox read -> FleetCandidate build -> SelectFleetWidthTopN pack).
func TestC536_007_WaveSeedPicksDisjointRepsNotRawTopWeight(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "alpha", 0.9, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "beta", 0.8, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "gamma", 0.7, []string{"y.go"})

	reps := triagecap.SelectWaveSeedTopN(evolveDir, 2)
	if len(reps) != 2 {
		t.Fatalf("expected 2 disjoint lane reps for count=2; got %d (%v)", len(reps), repIDs(reps))
	}

	got := repIDs(reps)
	// alpha must be the highest-weight rep and lead the set.
	if got[0] != "alpha" {
		t.Errorf("highest-weight candidate must lead the seed; got order %v", got)
	}
	// beta must NOT appear — it collides with alpha on x.go (the anti-no-op).
	for _, id := range got {
		if id == "beta" {
			t.Errorf("beta shares x.go with alpha and must be rejected as a concurrent lane; got %v", got)
		}
	}
	// gamma (disjoint on y.go) must be the second lane.
	if got[1] != "gamma" {
		t.Errorf("disjoint runner-up gamma must be the second lane; got %v", got)
	}

	// Cross-check disjointness structurally: no file is owned by two reps.
	owner := map[string]string{}
	for _, r := range reps {
		for _, f := range r.Files {
			if prev, dup := owner[f]; dup {
				t.Errorf("reps %q and %q both own file %q — lanes are not file-disjoint", prev, r.ID, f)
			}
			owner[f] = r.ID
		}
	}
}

// TestC536_008_WaveSeedCountBelowTwoIsLegacySingleFocus is the edge case /
// backward-compat pin: count<2 reproduces the legacy single-focus selection —
// exactly the one highest-weight candidate, independent of file overlap. This
// mirrors SelectFleetWidthTopN's own count<2 contract through the wired seam.
func TestC536_008_WaveSeedCountBelowTwoIsLegacySingleFocus(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "alpha", 0.9, []string{"x.go"})
	writeInboxTodo(t, evolveDir, "beta", 0.8, []string{"x.go"})

	reps := triagecap.SelectWaveSeedTopN(evolveDir, 1)
	if len(reps) != 1 {
		t.Fatalf("count<2 must return exactly one candidate; got %d (%v)", len(reps), repIDs(reps))
	}
	if reps[0].ID != "alpha" {
		t.Errorf("count<2 must return the single highest-weight candidate; got %q", reps[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Shared — no-regression hygiene
// ---------------------------------------------------------------------------

// TestC536_009_TouchedPackagesBuildAndVetClean is the no-regression guard for
// both lanes (Acceptance Criteria Summary: `go build`/`go vet` clean). It builds
// the two production packages that gain new functions plus the command package
// that wires them, and vets the two library packages — all as real subprocesses.
// Building cmd/evolve proves the Task-3 wiring (in package main) compiles.
func TestC536_009_TouchedPackagesBuildAndVetClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	failurelogPkg := filepath.Join(root, "go", "internal", "failurelog")
	triagecapPkg := filepath.Join(root, "go", "internal", "triagecap")
	cmdPkg := filepath.Join(root, "go", "cmd", "evolve")

	for _, pkg := range []string{failurelogPkg, triagecapPkg, cmdPkg} {
		_, stderr, code, err := acsassert.SubprocessOutput("go", "build", pkg)
		if err != nil {
			t.Fatalf("failed to launch go build %s: %v", pkg, err)
		}
		if code != 0 {
			t.Errorf("go build %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
	}

	for _, pkg := range []string{failurelogPkg, triagecapPkg} {
		_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", pkg)
		if err != nil {
			t.Fatalf("failed to launch go vet %s: %v", pkg, err)
		}
		if code != 0 {
			t.Errorf("go vet %s must be clean; exit=%d stderr:\n%s", pkg, code, stderr)
		}
	}
}
