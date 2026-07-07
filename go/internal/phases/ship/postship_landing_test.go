// postship_landing_test.go — RED tests for cycle-609 task
// fix-inbox-promotion-landing-gate.
//
// Cycle 598 defect (inbox-promotion-requires-landed-ship.json): a ship push
// was rejected (origin diverged), the recovery path reclassified to
// needs-reaudit, the cycle still reported FinalVerdict PASS, and
// promoteInbox promoted the inbox item to processed/ anyway — even though
// `git log --all -S` over the touched path showed nothing ever landed on
// any ref. Root cause: promoteInbox's only promotion criterion is
// "triage-decision.json is non-nil"; it never asks whether res.CommitSHA
// actually reached main/origin.
//
// Fix under test (not yet implemented — these tests MUST fail RED until
// Builder wires a landing check, reusing the existing isAncestor helper
// from repair.go, into promoteInbox's Promote/ReconcileSuperseded calls):
// an unlanded commit SHA must skip promotion for BOTH the primary
// top_n/skip_shipped loop and the superseded-reconcile path, logging a
// "[ship] WARN: promotion skipped: unlanded" line instead of "promoted:
// landed", and the item must NOT appear under processed/cycle-<cid>/.
package ship

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// landingScriptedRunner scripts "git merge-base" (the isAncestor call
// signature: merge-base --is-ancestor <sha> HEAD) to report landed/unlanded.
func landingScriptedRunner(mergeBaseExit int) *scriptedRunner {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git merge-base"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{exit: mergeBaseExit}
	return r
}

// writeInboxFixture lays down .evolve/inbox/processing/cycle-<cid>/<id>.json,
// .evolve/runs/cycle-<cid>/triage-decision.json (top_n: [id]), and
// .evolve/cycle-state.json:cycle_id=<cid>. Returns the processing-dir path.
func writeInboxFixture(t *testing.T, root string, cid int, id string) string {
	t.Helper()
	procDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+itoa(cid))
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatalf("mkdir processing: %v", err)
	}
	item := map[string]any{"id": id, "title": "fixture item"}
	body, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(procDir, id+".json"), body, 0o644); err != nil {
		t.Fatalf("write inbox item: %v", err)
	}

	cycleDir := filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cid))
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir cycleDir: %v", err)
	}
	decision := map[string]any{
		"top_n": []map[string]any{{"id": id}},
	}
	dbody, _ := json.Marshal(decision)
	if err := os.WriteFile(filepath.Join(cycleDir, "triage-decision.json"), dbody, 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}

	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{
		"cycle_id": float64(cid),
	})
	return procDir
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// TestPromoteInbox_UnlandedCommitSkipsPromotion is the primary RED case:
// res.CommitSHA is neither an ancestor of HEAD nor found on origin (fake
// runner reports merge-base --is-ancestor exit!=0 for every ref probed).
// promoteInbox must NOT call Promote(..., "processed", ...) — the item
// stays out of processed/cycle-<cid>/ — and must log the unlanded WARN.
func TestPromoteInbox_UnlandedCommitSkipsPromotion(t *testing.T) {
	root := t.TempDir()
	const cid = 609
	const id = "fix-inbox-promotion-landing-gate"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(1) // not an ancestor of HEAD or origin
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	processedDir := filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+itoa(cid))
	entries, _ := os.ReadDir(processedDir)
	if len(entries) != 0 {
		t.Errorf("unlanded commit must not promote to processed/; found %d entries: %v", len(entries), entries)
	}
	if !anyContains(res.Logs, "promotion skipped: unlanded") {
		t.Errorf("expected 'promotion skipped: unlanded' WARN log; got %v", res.Logs)
	}
}

// TestPromoteInbox_LandedCommitPromotes is the twin GREEN case: the exact
// same fixture, but res.CommitSHA IS an ancestor of HEAD (merge-base
// --is-ancestor exits 0). promoteInbox must promote normally and log
// "promoted: landed".
func TestPromoteInbox_LandedCommitPromotes(t *testing.T) {
	root := t.TempDir()
	const cid = 609
	const id = "fix-inbox-promotion-landing-gate"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(0) // is an ancestor of HEAD
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "cafebabecafebabecafebabecafebabecafebabe"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	processedDir := filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+itoa(cid))
	entries, _ := os.ReadDir(processedDir)
	if len(entries) == 0 {
		t.Errorf("landed commit must promote to processed/cycle-%d/; found none", cid)
	}
	if !anyContains(res.Logs, "promoted: landed") {
		t.Errorf("expected 'promoted: landed' log; got %v", res.Logs)
	}
}

// TestPromoteInbox_ReconcileSuperseded_UnlandedSkipsRetirement: the
// superseded-reconcile path (postship.go:133, ReconcileSuperseded) shares
// the same defect and must get the identical landing check — an unlanded
// commit must not retire a superseded id either.
func TestPromoteInbox_ReconcileSuperseded_UnlandedSkipsRetirement(t *testing.T) {
	root := t.TempDir()
	const cid = 609
	const supersededID = "loop-self-prioritize-unmet-fleet-concurrency"
	procDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+itoa(cid))
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	item := map[string]any{"id": supersededID}
	body, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(procDir, supersededID+".json"), body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cycleDir := filepath.Join(root, ".evolve", "runs", "cycle-"+itoa(cid))
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	decision := map[string]any{
		"top_n":      []map[string]any{{"id": "recover-ship-fleet-starvation-observer"}},
		"superseded": []string{supersededID},
	}
	dbody, _ := json.Marshal(decision)
	if err := os.WriteFile(filepath.Join(cycleDir, "triage-decision.json"), dbody, 0o644); err != nil {
		t.Fatalf("write triage-decision.json: %v", err)
	}
	mustWriteState(t, filepath.Join(root, ".evolve", "cycle-state.json"), map[string]any{
		"cycle_id": float64(cid),
	})
	// The primary top_n item's own fixture file is absent on purpose — this
	// test isolates the superseded-reconcile branch; Promote's ship.sh-compat
	// NoOp on a missing file is a pre-existing, unrelated pass.

	r := landingScriptedRunner(1) // unlanded
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "0000000000000000000000000000000000000000"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	processedDir := filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+itoa(cid))
	if _, statErr := os.Stat(filepath.Join(processedDir, supersededID+".json")); statErr == nil {
		t.Errorf("unlanded commit must not retire superseded id %q via ReconcileSuperseded", supersededID)
	}
	if !anyContains(res.Logs, "promotion skipped: unlanded") {
		t.Errorf("expected unlanded WARN log covering the superseded-reconcile path too; got %v", res.Logs)
	}
}

// TestPromoteInbox_NeedsReauditOutcomeNeverPromotes is the cycle-598
// regression itself: RepairOutcome=="needs-reaudit" (origin diverged,
// repairPushRace declined to push) paired with a CommitSHA that is only a
// local commit (not an ancestor of HEAD-on-origin — modeled here as
// merge-base --is-ancestor failing) must never promote, regardless of
// whether a caller upstream still considers the cycle a "PASS". The landing
// check is the single source of truth, independent of verdict labels.
func TestPromoteInbox_NeedsReauditOutcomeNeverPromotes(t *testing.T) {
	root := t.TempDir()
	const cid = 598
	const id = "skill-overlays-bridge-layer"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(1)
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{
		CommitSHA:     "1111111111111111111111111111111111111111",
		RepairOutcome: "needs-reaudit",
	}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	processedDir := filepath.Join(root, ".evolve", "inbox", "processed", "cycle-"+itoa(cid))
	entries, _ := os.ReadDir(processedDir)
	if len(entries) != 0 {
		t.Errorf("needs-reaudit outcome with an unlanded commit must never promote; found %v", entries)
	}
	if !anyContains(res.Logs, "promotion skipped: unlanded") {
		t.Errorf("expected unlanded WARN log for the needs-reaudit regression case; got %v", res.Logs)
	}
}
