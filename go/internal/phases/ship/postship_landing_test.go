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

func TestPromoteInbox_UnlandedCommitSkipsPromotion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 609
	const id = "fix-inbox-promotion-landing-gate"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(1)
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

func TestPromoteInbox_LandedCommitPromotes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 609
	const id = "fix-inbox-promotion-landing-gate"
	writeInboxFixture(t, root, cid, id)

	r := landingScriptedRunner(0)
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

func TestPromoteInbox_ReconcileSuperseded_UnlandedSkipsRetirement(t *testing.T) {
	t.Parallel()
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

	r := landingScriptedRunner(1)
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

func TestPromoteInbox_NeedsReauditOutcomeNeverPromotes(t *testing.T) {
	t.Parallel()
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
