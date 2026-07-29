//go:build acs

// Package cycle1180 materialises the cycle-1180 acceptance criteria for the two
// triage-COMMITTED (## top_n) fleet-scoped tasks pinned to this lane:
//
//   - wave-lane-task-quarantine-dead   → a FAILED WAVE LANE must move its
//     triage-committed ids through the ADR-0072 S5 failure lifecycle
//     (failure_count bump → quarantine at ceiling), and a quarantined id must
//     then read as CONSUMED at dispatch so it stops being re-picked.
//   - wave-planner-pass-scope-prune    → the wave SEED must drop carried-over
//     committed ids that the inbox lifecycle has already consumed.
//
// (workspace-hygiene-s5-wiring-shadow-default was DROPPED by triage as
// already-landed — no predicate here, per R9.3 predicates-bind-to-committed-work.)
//
// What is actually broken today (verified in this worktree, not assumed):
//
//  1. cmd_loop.go's WAVE branch (cmd_loop.go:596-604) only COUNTS failed lanes
//     and logs "N/M lanes ok". The sequential branch (cmd_loop.go:722-732) is
//     the ONLY caller of inboxmover.ApplyCycleOutcome's FAIL path, and
//     fleet.Result{Index,ExitCode,Err} carries neither the lane's cycle number
//     nor its workspace — so nothing can apply a lane's verdict from there.
//     Fleet-dispatched work therefore never bumps failure_count and the S5
//     retry ceiling is structurally unreachable (batch-14: cycles 1137/1139/
//     1142/1143 all FAILed on the same ids with failure_count stuck at 0).
//     The PASS half already lives INSIDE the cycle process
//     (internal/phases/ship/postship.go:188) — the FAIL half must be its
//     symmetric, equally importable sibling. Predicates 002/003 pin that seam.
//  2. inboxmover.ResolveDispatchState (dispatchstate.go:41-58) classifies
//     inbox root / processing / processed / rejected / retry — but NOT
//     quarantine/. A quarantined id falls through to StateUnknown, which the
//     dispatch freshness gate fails OPEN on (cmd_loop_wave.go:323-324) — so
//     quarantine, even once reachable, would not stop re-dispatch. Predicate
//     001 pins it.
//  3. triagecap.WidenTopNToFleetWidth (topn_width.go:96-97) copies the
//     carried-over `committed` slice through VERBATIM, and SelectWaveSeedMenus
//     (lane_menu.go:103-109) hands it straight to the wave plan. An id already
//     consumed at an earlier wave is re-pinned into a later lane-scope.json —
//     the cycle-1116 re-pin of tdd-topn-binding-gate (consumed at cycle-1113).
//     Predicate 004 pins it.
//
// Predicate strategy — every predicate DRIVES the system under test over an
// isolated temp tree and asserts on the resulting on-disk lifecycle state or
// return value. There is not one source-grep assertion in this file (the
// cycle-85 degenerate-predicate ban): adding a magic string to a source file
// greens nothing here.
//
// Diversity: 001 asserts a state classification, 002 the positive bump +
// quarantine transition, 003 the NEGATIVE half (uncommitted menu ids and
// system-level failures must stay inert — the anti-over-quarantine guard), 004
// the prune plus its fail-open edge (an id with no lifecycle evidence at all
// must survive).
package cycle1180

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// --- fixture helpers --------------------------------------------------------

// newProject builds an isolated project root with an empty .evolve/inbox/ and
// returns (projectRoot, inboxDir). Every predicate gets its own tree: the
// lifecycle is filesystem-shaped, so a shared root would let one predicate's
// moves leak into another's assertions.
func newProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

// writeItem drops an inbox item JSON carrying id (plus optional weight and a
// pre-existing failure_count) into dir, mirroring .evolve/inbox/ naming.
func writeItem(t *testing.T, dir, id string, failureCount int, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{"id": id, "title": "fixture item " + id, "kind": "bug"}
	if failureCount > 0 {
		doc["failure_count"] = failureCount
	}
	if weight > 0 {
		doc["weight"] = weight
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	path := filepath.Join(dir, "2026-07-29T00-00-00Z-"+id+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
	return path
}

// writeTriageDecision writes the workspace's triage-decision.json — the ONE
// artifact inboxmover.CommittedIDs reads to learn "what this cycle worked".
// The failure seam must key off this file, not off the lane's whole menu.
func writeTriageDecision(t *testing.T, workspace string, topN []string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	rows := make([]map[string]string, 0, len(topN))
	for _, id := range topN {
		rows = append(rows, map[string]string{"id": id})
	}
	body, err := json.MarshalIndent(map[string]any{"top_n": rows}, "", "  ")
	if err != nil {
		t.Fatalf("marshal triage decision: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "triage-decision.json"), body, 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}
}

// testOpts roots inboxmover at root with the landing gate stubbed to "landed":
// the real gate shells out to git, which is noise in a temp dir.
func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

// findItem returns the path of the file directly under dir whose JSON .id == id,
// or "". Non-recursive by design: each lifecycle destination is a flat dir.
func findItem(t *testing.T, dir, id string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		body, rErr := os.ReadFile(path)
		if rErr != nil {
			continue
		}
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(body, &doc) == nil && doc.ID == id {
			return path
		}
	}
	return ""
}

// failureCountOf reads the durable failure_count off an item JSON; absent is 0,
// the same reading bumpFailureCount uses.
func failureCountOf(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return 0
	}
	return doc.FailureCount
}

// ids flattens lane menus to a flat id set for membership assertions.
func ids(menus [][]triagecap.FleetCandidate) map[string]bool {
	out := map[string]bool{}
	for _, menu := range menus {
		for _, c := range menu {
			out[c.ID] = true
		}
	}
	return out
}

// --- 001: quarantined ids read as CONSUMED at dispatch ----------------------

// TestC1180_001_QuarantinedIdIsConsumedAtDispatch drives the real dispatch-state
// resolver over a tree whose only evidence for the id is .evolve/inbox/quarantine/.
//
// Today ResolveDispatchState never looks in quarantine/, so it returns
// StateUnknown — and the wave freshness gate treats unknown as FRESH (fail-open,
// cmd_loop_wave.go:323). That makes quarantine cosmetic: a poison todo parked by
// the S5 ceiling is still launchable. The criterion is that quarantine is a
// CONSUMED lifecycle state, reported as such (state string "quarantine") so the
// gate's default branch skips the lane with a legible reason.
//
// Negative half in the same predicate: an id with NO lifecycle evidence anywhere
// must STILL resolve unknown — the fail-open posture that keeps non-inbox-backed
// planned ids launchable must not be collateral damage of the fix.
func TestC1180_001_QuarantinedIdIsConsumedAtDispatch(t *testing.T) {
	root, inbox := newProject(t)
	writeItem(t, filepath.Join(inbox, "quarantine"), "poison-todo", 3, 0.9)

	ds := inboxmover.ResolveDispatchState(testOpts(root, io.Discard), "poison-todo")

	if ds.State == inboxmover.StatePending || ds.State == inboxmover.StateUnknown {
		t.Errorf("ResolveDispatchState(quarantined id).State = %q; want a CONSUMED state (\"quarantine\") — pending/unknown both fail the freshness gate OPEN, so the ADR-0072 S5 ceiling never stops re-dispatch", ds.State)
	}
	if ds.State != "quarantine" {
		t.Errorf("ResolveDispatchState(quarantined id).State = %q; want \"quarantine\" so the gate's skip reason names the real cause", ds.State)
	}

	// Fail-open must survive: no evidence anywhere ⇒ unknown ⇒ launchable.
	if got := inboxmover.ResolveDispatchState(testOpts(root, io.Discard), "never-filed").State; got != inboxmover.StateUnknown {
		t.Errorf("ResolveDispatchState(id with no lifecycle evidence).State = %q; want %q — a planned id that is not inbox-backed must never be false-skipped", got, inboxmover.StateUnknown)
	}
}

// --- 002: a FAILED lane walks its committed ids toward quarantine -----------

// TestC1180_002_LaneFailureBumpsAndQuarantines is the cycle-1180 CRUX for
// wave-lane-task-quarantine-dead.
//
// It drives the importable failure-closeout seam — the symmetric sibling of the
// PASS half already living inside the cycle process (phases/ship/postship.go) —
// over a temp project, three times in a row against a ceiling of 3, and asserts
// the durable lifecycle actually moves:
//
//	FAIL #1 → failure_count 1, released to the inbox root (still re-pickable)
//	FAIL #2 → failure_count 2, still at the root
//	FAIL #3 → at the ceiling ⇒ the item is in .evolve/inbox/quarantine/, GONE
//	          from the root, and reported in OutcomeResult.Quarantined
//
// A seam that merely releases (today's whole-batch behavior) fails at FAIL #1;
// one that bumps but never routes to quarantine fails at FAIL #3. Nothing here
// passes on an unfixed tree, and no source string can green it.
func TestC1180_002_LaneFailureBumpsAndQuarantines(t *testing.T) {
	root, inbox := newProject(t)
	writeItem(t, inbox, "poison-todo", 0, 0.9)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1180")
	writeTriageDecision(t, workspace, []string{"poison-todo"})

	const ceiling = 3
	for attempt := 1; attempt <= ceiling; attempt++ {
		res, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
			ProjectRoot: root,
			Workspace:   workspace,
			Cycle:       1180,
			Ceiling:     ceiling,
			SystemLevel: false,
			Reason:      "cycle-failure-release",
			Stderr:      io.Discard,
		})
		if err != nil {
			t.Fatalf("attempt %d: ApplyFailure returned %v; want nil — a lane FAIL must always reach the lifecycle", attempt, err)
		}

		if attempt < ceiling {
			path := findItem(t, inbox, "poison-todo")
			if path == "" {
				t.Fatalf("attempt %d: 'poison-todo' is not at the inbox root; below the ceiling a failed item must be released for re-pick", attempt)
			}
			if got := failureCountOf(t, path); got != attempt {
				t.Errorf("attempt %d: failure_count = %d; want %d — an unbumped counter is exactly the batch-14 defect (four FAILs, count stuck at 0)", attempt, got, attempt)
			}
			continue
		}

		// At the ceiling: quarantined, not released.
		if path := findItem(t, inbox, "poison-todo"); path != "" {
			t.Errorf("attempt %d (ceiling %d): 'poison-todo' is STILL at the inbox root (%s); at the ceiling it must be parked in quarantine/ so it stops being re-picked", attempt, ceiling, filepath.Base(path))
		}
		if findItem(t, filepath.Join(inbox, "quarantine"), "poison-todo") == "" {
			t.Errorf("attempt %d (ceiling %d): 'poison-todo' is not in .evolve/inbox/quarantine/ — the ADR-0072 S5 ceiling is unreachable for fleet-dispatched work", attempt, ceiling)
		}
		if len(res.Quarantined) == 0 {
			t.Errorf("attempt %d: OutcomeResult.Quarantined is empty; want the quarantined path reported so the caller can log it", attempt)
		}
	}
}

// --- 003: menu semantics + system-level failures stay INERT (negative) ------

// TestC1180_003_UncommittedMenuAndSystemFailuresStayInert is the anti-over-
// quarantine half of wave-lane-task-quarantine-dead (its menu-semantics guard).
//
// Since PR #366 a wave lane CLAIMS a whole menu but triage commits only a subset.
// Bumping the whole menu on FAIL would walk healthy backlog toward quarantine on
// failures of an unrelated task — the exact inverse defect. Two negatives:
//
//	(a) an id present in the lane's inbox but ABSENT from triage-decision.json's
//	    top_n must end with failure_count 0 and stay at the inbox root;
//	(b) a SYSTEM-level failure (ADR-0072 S3 — quota storm, forged verdict) must
//	    bump NOTHING, not even the committed id: it is not the task's fault.
func TestC1180_003_UncommittedMenuAndSystemFailuresStayInert(t *testing.T) {
	// (a) uncommitted menu id is inert on a task-level FAIL.
	root, inbox := newProject(t)
	writeItem(t, inbox, "worked-todo", 0, 0.9)
	writeItem(t, inbox, "menu-only-todo", 0, 0.8)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1180")
	writeTriageDecision(t, workspace, []string{"worked-todo"})

	if _, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
		ProjectRoot: root,
		Workspace:   workspace,
		Cycle:       1180,
		Ceiling:     1, // ceiling 1: any bump quarantines, making leakage loud
		SystemLevel: false,
		Reason:      "cycle-failure-release",
		Stderr:      io.Discard,
	}); err != nil {
		t.Fatalf("ApplyFailure (task-level): %v", err)
	}

	menuPath := findItem(t, inbox, "menu-only-todo")
	if menuPath == "" {
		t.Fatalf("'menu-only-todo' left the inbox root; an id no phase worked must never move")
	}
	if got := failureCountOf(t, menuPath); got != 0 {
		t.Errorf("uncommitted 'menu-only-todo' failure_count = %d; want 0 — only triage-COMMITTED ids may accrue task-level failures (PR #366 menu semantics)", got)
	}
	if findItem(t, filepath.Join(inbox, "quarantine"), "menu-only-todo") != "" {
		t.Errorf("uncommitted 'menu-only-todo' was QUARANTINED; healthy backlog must not be poisoned by an unrelated task's FAIL")
	}
	if findItem(t, filepath.Join(inbox, "quarantine"), "worked-todo") == "" {
		t.Errorf("committed 'worked-todo' was NOT quarantined at ceiling 1; the committed id is precisely the one that must move")
	}

	// (b) system-level failure bumps nothing at all.
	sysRoot, sysInbox := newProject(t)
	writeItem(t, sysInbox, "worked-todo", 0, 0.9)
	sysWorkspace := filepath.Join(sysRoot, ".evolve", "runs", "cycle-1181")
	writeTriageDecision(t, sysWorkspace, []string{"worked-todo"})

	if _, err := cycleoutcome.ApplyFailure(cycleoutcome.FailureInputs{
		ProjectRoot: sysRoot,
		Workspace:   sysWorkspace,
		Cycle:       1181,
		Ceiling:     1,
		SystemLevel: true,
		Reason:      "system-failure-release",
		Stderr:      io.Discard,
	}); err != nil {
		t.Fatalf("ApplyFailure (system-level): %v", err)
	}
	sysPath := findItem(t, sysInbox, "worked-todo")
	if sysPath == "" {
		t.Fatalf("'worked-todo' left the inbox root on a SYSTEM-level failure; S3 failures release, never quarantine (ADR-0072 AC4)")
	}
	if got := failureCountOf(t, sysPath); got != 0 {
		t.Errorf("system-level failure bumped failure_count to %d; want 0 — a quota/infra storm must not walk healthy ids toward the ceiling", got)
	}
}

// --- 004: the wave seed prunes already-consumed carried-over ids ------------

// TestC1180_004_WaveSeedPrunesConsumedCommittedIds pins
// wave-planner-pass-scope-prune against the real seed seam.
//
// SelectWaveSeedMenus takes the prior decision's `committed` candidates and
// (via WidenTopNToFleetWidth) copies them into the wave plan verbatim. An id
// consumed at an earlier wave is therefore re-pinned into a later
// lane-scope.json — cycle-1116 re-pinned tdd-topn-binding-gate after cycle-1113
// consumed it. The criterion: the SEED itself must drop candidates the inbox
// lifecycle has already consumed (processed/, and quarantine/ once 001 lands),
// so the plan artifact is honest rather than relying on the launch-time gate.
//
// Fail-open edge, asserted in the same predicate: a carried-over id with NO
// lifecycle evidence at all (not inbox-backed — a synthetic or externally
// sourced card) must SURVIVE the prune. A prune that drops everything it cannot
// resolve would starve every wave, so this negative is load-bearing.
func TestC1180_004_WaveSeedPrunesConsumedCommittedIds(t *testing.T) {
	root, inbox := newProject(t)
	evolveDir := filepath.Join(root, ".evolve")

	// Pending backlog (the widen source) + the two consumed carry-overs.
	writeItem(t, inbox, "fresh-a", 0, 0.9)
	writeItem(t, inbox, "fresh-b", 0, 0.8)
	writeItem(t, filepath.Join(inbox, "processed"), "consumed-todo", 0, 0.95)
	writeItem(t, filepath.Join(inbox, "quarantine"), "poison-todo", 3, 0.97)

	committed := []triagecap.FleetCandidate{
		{ID: "consumed-todo", Weight: 0.95, Files: []string{"go/internal/a/a.go"}},
		{ID: "poison-todo", Weight: 0.97, Files: []string{"go/internal/b/b.go"}},
		{ID: "fresh-a", Weight: 0.9, Files: []string{"go/internal/c/c.go"}},
		{ID: "not-inbox-backed", Weight: 0.5, Files: []string{"go/internal/d/d.go"}},
	}

	got := ids(triagecap.SelectWaveSeedMenus(evolveDir, committed, 3, 4, nil))

	if got["consumed-todo"] {
		t.Errorf("wave seed re-pinned 'consumed-todo' (already in inbox/processed/) — this is the cycle-1116 re-pin: a consumed id must never re-enter a wave plan")
	}
	if got["poison-todo"] {
		t.Errorf("wave seed re-pinned 'poison-todo' (in inbox/quarantine/) — quarantine must remove an id from the planner's supply, not just from the launcher's")
	}
	if !got["fresh-a"] {
		t.Errorf("wave seed dropped still-pending 'fresh-a'; the prune must only remove CONSUMED ids, never live committed work")
	}
	if !got["not-inbox-backed"] {
		t.Errorf("wave seed dropped 'not-inbox-backed' (no lifecycle evidence anywhere); the prune must fail OPEN or every non-inbox-backed card starves its wave")
	}
}
