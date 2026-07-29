package inboxmover

// root_failure_test.go — ADR-0080 P2: FAIL-side attempt accounting for
// ROOT-RESIDENT items. Wave lanes never claim into processing/, so the
// existing release-path bump+quarantine (ADR-0072 S5) is structurally
// unreachable for graded FAILs: workspace-hygiene burned 12 lanes and
// quarantine-dead 7 with failure_count still 0. RecordRootTaskFailure is the
// root-resident twin: bump the durable counter where the item actually
// lives; at the ceiling, move it to the terminal quarantine/ dir.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func rootFailureFixture(t *testing.T, id string, failureCount int) Options {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	item := map[string]any{"id": id, "weight": 0.9}
	if failureCount > 0 {
		item["failure_count"] = failureCount
	}
	b, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "2026-07-28T00-00-00Z-"+id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{ProjectRoot: root, Stderr: io.Discard}
}

func TestRecordRootTaskFailure_BumpsDurableCounter(t *testing.T) {
	opts := rootFailureFixture(t, "task-a", 0)
	count, quarantined, err := RecordRootTaskFailure(opts, "task-a", 1201, "audit verdict FAIL", 3)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || quarantined {
		t.Fatalf("count=%d quarantined=%v, want 1/false", count, quarantined)
	}
	opts.resolveOpts()
	raw, err := os.ReadFile(filepath.Join(opts.InboxDir, "2026-07-28T00-00-00Z-task-a.json"))
	if err != nil {
		t.Fatalf("item must remain in the root below the ceiling: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["failure_count"].(float64) != 1 {
		t.Fatalf("durable failure_count = %v, want 1", m["failure_count"])
	}
}

func TestRecordRootTaskFailure_QuarantinesAtCeiling(t *testing.T) {
	opts := rootFailureFixture(t, "task-b", 2)
	count, quarantined, err := RecordRootTaskFailure(opts, "task-b", 1202, "audit verdict FAIL", 3)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || !quarantined {
		t.Fatalf("count=%d quarantined=%v, want 3/true — the ceiling must actually fire (12-lane grind pin)", count, quarantined)
	}
	opts.resolveOpts()
	if _, err := os.Stat(filepath.Join(opts.InboxDir, "quarantine", "2026-07-28T00-00-00Z-task-b.json")); err != nil {
		t.Fatalf("item not in terminal quarantine/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(opts.InboxDir, "2026-07-28T00-00-00Z-task-b.json")); err == nil {
		t.Fatal("quarantined item must leave the dispatchable root")
	}
}

func TestRecordRootTaskFailure_UnknownIDIsQuietNoop(t *testing.T) {
	opts := rootFailureFixture(t, "task-c", 0)
	count, quarantined, err := RecordRootTaskFailure(opts, "no-such-task", 1203, "x", 3)
	if err != nil || count != 0 || quarantined {
		t.Fatalf("unknown id must no-op (scout-originated work has no inbox item): %d %v %v", count, quarantined, err)
	}
}

func TestRecordRootTaskFailure_ZeroCeilingNeverQuarantines(t *testing.T) {
	opts := rootFailureFixture(t, "task-d", 9)
	count, quarantined, err := RecordRootTaskFailure(opts, "task-d", 1204, "x", 0)
	if err != nil || !(count == 10) || quarantined {
		t.Fatalf("ceiling 0 = disabled (policy escape hatch): %d %v %v", count, quarantined, err)
	}
}
