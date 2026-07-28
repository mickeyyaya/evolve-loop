//go:build acs

// Package cycle1156 materialises the acceptance criteria for the three tasks
// triage COMMITTED to this fleet lane (triage-report.md `## top_n`):
//
//   - inboxmover-promote-mkdir-fail-loud  → 001, 002, 003
//   - wave-lane-task-quarantine-dead      → 004, 005, 006
//   - menu-pass-promotes-committed-ids    → 007, 008
//
// The fourth lane-scope id (workspace-hygiene-s5-wiring-shadow-default) was
// DEFERRED by triage and therefore gets ZERO predicates here (R9.3 floor-binding:
// predicates bind only to triage-committed work; a deferred-floor predicate
// starves the committed tasks — cycle-280).
//
// # Why these three are one lifecycle contract
//
// The inbox item for wave-lane-task-quarantine-dead states it explicitly: "one
// lifecycle seam handling PASS-promote + FAIL-bump covers both". Today there are
// two half-lifecycles and a hole in the middle:
//
//   - PASS side (menu-pass-promotes-committed-ids): promotion is agent-driven, so
//     cycle-1147 shipped three menu items in one commit and promoted NONE of them
//     — processed/cycle-1147/ is empty and all three re-entered the backlog.
//   - FAIL side (wave-lane-task-quarantine-dead): the failure drain is the only
//     bumpFailureCount caller and it walks ONLY
//     processing/cycle-N/. Wave lanes never claim their ids into processing/, so
//     the ADR-0072 S5 retry ceiling is structurally unreachable for fleet work
//     (batch-14: four FAILs, failure_count never incremented).
//   - And when the underlying move fails, Promote (inboxmover.go:305-318) reports
//     the infrastructure failure as NoOp=true / nil error — the ship.sh "already
//     done" compat contract reused for a genuine non-delivery.
//
// # Contract pinned by these predicates (Builder: implement these exact signatures)
//
// Per Core Rule 5 (deterministic work belongs in code, not agent instructions),
// predicates 004-008 pin ONE exported lifecycle seam in package inboxmover rather
// than two parallel models (never_duplicate_centralize):
//
//	type CycleOutcome struct {
//	    Cycle        int      // cycle number
//	    Passed       bool     // true = PASS (promote), false = FAIL (bump/quarantine)
//	    CommittedIDs []string // triage-decision.json `## top_n` — the worked set
//	    CommitSHA    string   // ship SHA, PASS only ("" = no SHA prefix)
//	    Reason       string   // ledger reason ("" = default)
//	    Ceiling      int      // FailureThresholds.TaskRetryCeiling (FAIL only)
//	    SystemLevel  bool     // S3 system failure: NEVER quarantines (AC4)
//	}
//	func ApplyCycleOutcome(opts Options, oc CycleOutcome) (OutcomeResult, error)
//	func ClaimLaneScope(opts Options, cycle int, ids []string) ([]string, error)
//
// The predicates deliberately assert on the FILESYSTEM end state (where the item
// physically lands, and what its durable failure_count says) rather than on the
// returned OutcomeResult — the on-disk lifecycle IS the contract, and the Builder
// keeps freedom over the result struct's shape.
//
// # Predicate quality (cycle-85 ban)
//
// Every predicate below CALLS the production function and asserts on its return
// value, its error, or the artifact it moved on disk; 003 runs the real `evolve`
// binary as a subprocess and asserts on its exit code. None of them is a
// source-grep for a magic string, so none can be satisfied by adding a comment.
package cycle1156

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- fixture helpers --------------------------------------------------------

// newInbox builds an isolated project root with an empty .evolve/inbox/ and
// returns (projectRoot, inboxDir). Every predicate runs against its own temp
// tree: the lifecycle is filesystem-shaped, so a shared root would let one
// predicate's moves leak into another's assertions.
func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

// writeItem drops an inbox item JSON carrying id (and an optional pre-existing
// failure_count) into dir, mirroring the real .evolve/inbox/ naming convention
// (<timestamp>-<id>.json). Returns the path written.
func writeItem(t *testing.T, dir, id string, failureCount int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	doc := map[string]any{
		"id":       id,
		"title":    "fixture item " + id,
		"kind":     "bug",
		"weight":   0.5,
		"priority": "medium",
	}
	if failureCount > 0 {
		doc["failure_count"] = failureCount
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal item %s: %v", id, err)
	}
	path := filepath.Join(dir, "2026-07-28T00-00-00Z-"+id+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item %s: %v", id, err)
	}
	return path
}

// testOpts returns inboxmover Options rooted at root with the landing gate
// stubbed to "landed". The real gate shells out to `git merge-base` and is
// fail-open on a non-git dir; stubbing it keeps the promote predicates asserting
// the LIFECYCLE rather than incidental git behaviour of a temp dir.
func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

// findItem returns the path of the file under dir whose JSON .id == id, or "".
// Non-recursive by design: each lifecycle destination is a single flat dir.
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

// failureCountOf reads the durable failure_count off an item JSON. Returns 0
// when the field is absent — the same reading bumpFailureCount uses, so "absent"
// and "0" are indistinguishable to the system and must be to the predicate too.
func failureCountOf(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item %s: %v", path, err)
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse item %s: %v", path, err)
	}
	return doc.FailureCount
}

// countItems returns how many *.json files live directly under dir (0 when the
// dir does not exist).
func countItems(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
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

// lockDir makes dir read-only (0555) so a child MkdirAll under it fails with
// EACCES, and restores 0755 at cleanup so t.TempDir() can remove the tree.
// Skips the whole predicate when running as root, where mode bits are advisory.
func lockDir(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory mode bits do not deny mkdir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod 0555 %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// --- Task: inboxmover-promote-mkdir-fail-loud -------------------------------

// AC1 (RED): "TestPromote_MkdirFailed_ReturnsError — read-only parent for
// destDir → err != nil, res.NoOp == false (fails today)".
//
// Today inboxmover.go:305-318 WARNs to a default-io.Discard stderr, writes a
// best-effort ledger line, and returns (PromoteResult{NoOp:true}, nil): a
// stranded task is indistinguishable from a completed one to every caller. The
// predicate also asserts the item is still findable at its source, because the
// only safe meaning of "mkdir failed" is "nothing moved" — a loud error that
// also lost the file would be a worse defect than the silent one.
func TestC1156_001_promote_mkdir_failure_returns_error(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "mkdir-fail-item", 0)
	lockDir(t, filepath.Join(inbox, "processed"))

	var stderr strings.Builder
	res, err := inboxmover.Promote(testOpts(root, &stderr), "mkdir-fail-item", "processed",
		inboxmover.PromoteOpts{Cycle: "1156"})

	if err == nil {
		t.Errorf("Promote returned nil error after mkdir of %s/processed/cycle-1156 failed: an infrastructure failure must not be reported as success (res=%+v)", inbox, res)
	}
	if res.NoOp {
		t.Errorf("Promote returned NoOp=true after a mkdir failure: NoOp is the ship.sh 'source already moved' compat contract and must never cover a could-not-complete move")
	}
	if findItem(t, inbox, "mkdir-fail-item") == "" {
		t.Errorf("item left the inbox root despite the destination mkdir failing — a failed promote must not lose the task")
	}
	if !strings.Contains(stderr.String(), "mkdir") {
		t.Errorf("no mkdir diagnostic on stderr; got:\n%s", stderr.String())
	}
}

// AC2 (twin, compat): "TestPromote_SourceAlreadyMoved_NoOpSuccess — existing
// compat behavior preserved."
//
// This is the anti-overcorrection predicate for 001: making mkdir loud must not
// make the genuine "already moved" case loud. Expected to be pre-existing GREEN
// once the package compiles; it FAILS if the Builder converts every WARN path
// into an error.
func TestC1156_002_promote_source_already_moved_stays_noop_success(t *testing.T) {
	root, _ := newInbox(t)

	res, err := inboxmover.Promote(testOpts(root, io.Discard), "never-existed-item", "processed",
		inboxmover.PromoteOpts{Cycle: "1156"})

	if err != nil {
		t.Errorf("Promote of an already-moved id returned error %v: the ship.sh compat contract requires NoOp success", err)
	}
	if !res.NoOp {
		t.Errorf("Promote of an already-moved id returned NoOp=false: want NoOp=true (compat)")
	}
}

// AC3 (RED, caller check): "ship-phase promotion failure appears in cycle
// diagnostics, not exit 0."
//
// Runs the REAL binary end to end: `evolve inbox-mover promote` against a
// project root whose processed/ dir denies mkdir. cmd_inbox_mover.go:70-77
// currently maps everything except ErrBadArgs/ErrBadState to `return 0`
// ("ship.sh compat: all other paths exit 0"), so a non-delivery exits 0 today
// with no operator-visible diagnostic. Behavioural by construction — a source
// edit that does not change the process exit code cannot satisfy it.
func TestC1156_003_promote_failure_surfaces_nonzero_exit(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "cli-mkdir-fail-item", 0)
	lockDir(t, filepath.Join(inbox, "processed"))
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	stdout, stderr, code, _ := acsSubprocess(t, "go", "run", "../../cmd/evolve",
		"inbox-mover", "promote", "cli-mkdir-fail-item", "processed", "1156")

	if code == 0 {
		t.Errorf("`evolve inbox-mover promote` exited 0 after the destination mkdir failed: a stranded task must surface as a non-zero exit, not ship.sh-compat success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	if !strings.Contains(stderr, "mkdir") {
		t.Errorf("no mkdir diagnostic in cycle-visible stderr; got:\n%s", stderr)
	}
}

// acsSubprocess wraps the subprocess call so a missing `go` toolchain skips
// rather than red-failing the suite (the ACS runner may execute on a bare
// export). Compilation errors from `go run` still surface as a non-zero code,
// which is why 003 additionally asserts the mkdir diagnostic.
func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code, err
}

// --- Task: wave-lane-task-quarantine-dead -----------------------------------

// AC (RED, root cause): wave lanes never claim their scope, so the FAIL drain
// iterates an empty processing/cycle-N/ and the S5 ceiling is unreachable.
//
// ClaimLaneScope is the dispatch-side half of the single lifecycle seam: it must
// move each resolvable lane-scope id from the inbox root into
// processing/cycle-<N>/ and tolerate ids it cannot resolve (an id already
// claimed by another wave, or absent) without failing the whole dispatch —
// partial claiming must never abort a lane launch.
func TestC1156_004_lane_scope_claim_moves_menu_ids_to_processing(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "lane-item-a", 0)
	writeItem(t, inbox, "lane-item-b", 0)

	claimed, err := inboxmover.ClaimLaneScope(testOpts(root, io.Discard), 1156,
		[]string{"lane-item-a", "lane-item-b", "lane-item-absent"})
	if err != nil {
		t.Fatalf("ClaimLaneScope returned error %v: an unresolvable id must be tolerated, not abort the lane dispatch", err)
	}

	if len(claimed) != 2 {
		t.Errorf("ClaimLaneScope claimed %d id(s) (%v); want exactly the 2 resolvable ones", len(claimed), claimed)
	}
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	for _, id := range []string{"lane-item-a", "lane-item-b"} {
		if findItem(t, procDir, id) == "" {
			t.Errorf("%s not found in processing/cycle-1156/ after ClaimLaneScope: unclaimed items make the FAIL-side failure_count structurally unreachable", id)
		}
		if findItem(t, inbox, id) != "" {
			t.Errorf("%s still at the inbox root after being claimed: the claim must be a move, not a copy (double-dispatch risk)", id)
		}
	}
}

// AC (RED, menu semantics): "a wave lane whose cycle FAILs leaves its committed
// item with failure_count+1" AND "unworked menu ids (not committed by triage)
// neither bump nor quarantine".
//
// The second half is the anti-overcorrection axis: claiming the whole menu at
// dispatch and then bumping everything in processing/cycle-N/ (the legacy
// whole-dir drain behaviour) would punish items no phase
// ever worked, quarantining healthy backlog after N unrelated lane failures.
func TestC1156_005_failed_cycle_bumps_only_committed_ids(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	writeItem(t, procDir, "committed-item", 0)
	writeItem(t, procDir, "menu-only-item", 0)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       false,
		CommittedIDs: []string{"committed-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL) returned error: %v", err)
	}

	committed := findItem(t, inbox, "committed-item")
	if committed == "" {
		t.Fatalf("committed-item not released back to the inbox root after a FAIL below the ceiling")
	}
	if got := failureCountOf(t, committed); got != 1 {
		t.Errorf("committed-item failure_count = %d after one FAILed cycle; want 1 — an un-bumped count makes the ADR-0072 S5 retry ceiling unreachable (batch-14: four FAILs, zero increments)", got)
	}

	menuOnly := findItem(t, inbox, "menu-only-item")
	if menuOnly == "" {
		t.Fatalf("menu-only-item not released back to the inbox root after a FAIL")
	}
	if got := failureCountOf(t, menuOnly); got != 0 {
		t.Errorf("menu-only-item failure_count = %d; want 0 — triage never committed it, so no phase worked it and it must not accrue task-level failures", got)
	}
}

// AC (RED, ceiling + AC4 edge): "at task_retry_ceiling the item moves to
// quarantine and is not re-seeded", and a SYSTEM-level failure never quarantines
// (ADR-0072 S3 precedence).
func TestC1156_006_committed_id_quarantines_at_ceiling(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	writeItem(t, procDir, "poison-item", 1) // one prior failure; ceiling 2 → this FAIL quarantines
	writeItem(t, procDir, "menu-only-item", 1)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       false,
		CommittedIDs: []string{"poison-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL at ceiling) returned error: %v", err)
	}

	quarantineDir := filepath.Join(inbox, "quarantine")
	if findItem(t, quarantineDir, "poison-item") == "" {
		t.Errorf("poison-item not in inbox/quarantine/ after reaching the retry ceiling")
	}
	if findItem(t, inbox, "poison-item") != "" {
		t.Errorf("poison-item re-seeded at the inbox root after quarantine: a quarantined task must stop being re-picked every cycle")
	}
	if findItem(t, quarantineDir, "menu-only-item") != "" {
		t.Errorf("menu-only-item quarantined: an uncommitted menu id must never be quarantined by another task's failure")
	}

	// AC4 edge: the same shape with SystemLevel=true must NOT quarantine — an S3
	// system failure is not the task's fault.
	root2, inbox2 := newInbox(t)
	proc2 := filepath.Join(inbox2, "processing", "cycle-1156")
	writeItem(t, proc2, "sysfail-item", 1)
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root2, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       false,
		CommittedIDs: []string{"sysfail-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
		SystemLevel:  true,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(system-level FAIL) returned error: %v", err)
	}
	if findItem(t, filepath.Join(inbox2, "quarantine"), "sysfail-item") != "" {
		t.Errorf("sysfail-item quarantined on a SYSTEM-level failure: ADR-0072 S3 precedence forbids it (AC4)")
	}
	if findItem(t, inbox2, "sysfail-item") == "" {
		t.Errorf("sysfail-item not released back to the inbox root on a system-level failure")
	}
}

// --- Task: menu-pass-promotes-committed-ids ---------------------------------

// AC (RED): "a PASSing cycle whose triage committed N ids leaves
// processed/cycle-<N>/ holding exactly those N items; uncommitted menu ids stay
// in inbox root."
//
// Cycle-1147's shape verbatim: a menu ships several items in ONE commit, so the
// promote must be driven by the committed-id set in code — not by an agent that
// promoted nothing and left all three items to be re-offered by the very next
// triage (the verified-stale-drop burn that cost cycles 1131 and 1134). The
// "exactly N" count assertion is what rejects a promote-the-whole-menu shortcut.
func TestC1156_007_passing_cycle_promotes_exactly_committed_ids(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	writeItem(t, procDir, "shipped-a", 0)
	writeItem(t, inbox, "shipped-b", 0) // still at root: promotion must not depend on a claim
	writeItem(t, inbox, "menu-only-item", 0)

	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       true,
		CommittedIDs: []string{"shipped-a", "shipped-b"},
		CommitSHA:    "77dfdbc9aa11bb22",
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(PASS) returned error: %v", err)
	}

	processedDir := filepath.Join(inbox, "processed", "cycle-1156")
	for _, id := range []string{"shipped-a", "shipped-b"} {
		if findItem(t, processedDir, id) == "" {
			t.Errorf("%s not in processed/cycle-1156/ after a PASSing menu ship: cycle-1147 shipped 3 items and promoted 0, so all 3 re-entered the backlog", id)
		}
		if findItem(t, inbox, id) != "" {
			t.Errorf("%s still at the inbox root after promotion: it will be re-offered by the next triage", id)
		}
	}
	if n := countItems(t, processedDir); n != 2 {
		t.Errorf("processed/cycle-1156/ holds %d item(s); want exactly the 2 committed ids", n)
	}
	if findItem(t, inbox, "menu-only-item") == "" {
		t.Errorf("menu-only-item left the inbox root on PASS: triage did not commit it, so it stays pending")
	}
}

// AC (RED, idempotence + FAIL-side non-regression): "promote of an
// already-processed id is a no-op WARN" and "a FAIL promotes nothing".
//
// Idempotence matters because the legacy agent-driven promote may still run
// alongside the code-driven one during the transition — the two must not
// double-move or error. The FAIL half is the negative axis: it proves the PASS
// promotion is gated on the verdict rather than fired unconditionally.
func TestC1156_008_pass_promote_idempotent_and_fail_promotes_nothing(t *testing.T) {
	root, inbox := newInbox(t)
	writeItem(t, inbox, "idem-item", 0)
	oc := inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       true,
		CommittedIDs: []string{"idem-item"},
		CommitSHA:    "77dfdbc9aa11bb22",
	}
	opts := testOpts(root, io.Discard)

	if _, err := inboxmover.ApplyCycleOutcome(opts, oc); err != nil {
		t.Fatalf("ApplyCycleOutcome(PASS) returned error: %v", err)
	}
	if _, err := inboxmover.ApplyCycleOutcome(opts, oc); err != nil {
		t.Errorf("second ApplyCycleOutcome(PASS) returned error %v: re-promoting an already-processed id must be an idempotent no-op WARN", err)
	}
	processedDir := filepath.Join(inbox, "processed", "cycle-1156")
	if n := countItems(t, processedDir); n != 1 {
		t.Errorf("processed/cycle-1156/ holds %d item(s) after two identical PASS applications; want 1 (no duplicate)", n)
	}

	// FAIL side: nothing is promoted to processed/.
	root2, inbox2 := newInbox(t)
	writeItem(t, filepath.Join(inbox2, "processing", "cycle-1156"), "failed-item", 0)
	if _, err := inboxmover.ApplyCycleOutcome(testOpts(root2, io.Discard), inboxmover.CycleOutcome{
		Cycle:        1156,
		Passed:       false,
		CommittedIDs: []string{"failed-item"},
		Reason:       "cycle-failure-release",
		Ceiling:      2,
	}); err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL) returned error: %v", err)
	}
	if n := countItems(t, filepath.Join(inbox2, "processed", fmt.Sprintf("cycle-%d", 1156))); n != 0 {
		t.Errorf("processed/cycle-1156/ holds %d item(s) after a FAILing cycle; want 0 — a FAIL promotes nothing", n)
	}
}
