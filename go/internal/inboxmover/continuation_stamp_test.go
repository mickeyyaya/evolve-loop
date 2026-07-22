package inboxmover

// continuation_stamp_test.go — ADR-0076 slice C (S3): the FAIL-release stamps
// each released item with the cycle's continuation manifest, TRANSACTIONALLY
// with the release itself (pipeline-forensics lesson: item consumption must be
// transactional with landing — a stamp that happens in a separate pass can be
// lost to a crash between them). No manifest ⇒ byte-identical release.
// Quarantine is terminal parking: a quarantined item sheds any stamp so a
// later operator revival starts fresh.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func stampFixture(t *testing.T, withManifest bool, failureCount int) (Options, int, string) {
	t.Helper()
	root := t.TempDir()
	cycle := 91
	procDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-91")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := map[string]any{"id": "task-a", "title": "x"}
	if failureCount > 0 {
		item["failure_count"] = failureCount
	}
	body, _ := json.Marshal(item)
	itemPath := filepath.Join(procDir, "task-a.json")
	if err := os.WriteFile(itemPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if withManifest {
		ws := filepath.Join(root, ".evolve", "runs", "cycle-91")
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := continuation.WriteManifest(ws, continuation.Continuation{
			Worktree: "/wt", Branch: "evolve/cycle-91", SnapshotSHA: "abc123",
			BaseSHA: "def456", Cycle: 91,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return Options{ProjectRoot: root}, cycle, filepath.Join(root, ".evolve", "inbox", "task-a.json")
}

func readItemContinuation(t *testing.T, path string) *continuation.Continuation {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read released item: %v", err)
	}
	var it struct {
		Continuation *continuation.Continuation `json:"continuation"`
	}
	if err := json.Unmarshal(body, &it); err != nil {
		t.Fatalf("parse released item: %v", err)
	}
	return it.Continuation
}

func TestReleaseCycleProcessing_StampsContinuationFromManifest(t *testing.T) {
	opts, cycle, released := stampFixture(t, true, 0)
	if _, err := ReleaseCycleProcessingWithReason(opts, cycle, "cycle-failure-release"); err != nil {
		t.Fatalf("release: %v", err)
	}
	c := readItemContinuation(t, released)
	if c == nil {
		t.Fatal("released item must carry the continuation stamp")
	}
	if c.SnapshotSHA != "abc123" || c.Cycle != 91 {
		t.Errorf("stamp fields: %+v", c)
	}
}

func TestReleaseCycleProcessing_NoManifestNoStamp(t *testing.T) {
	opts, cycle, released := stampFixture(t, false, 0)
	if _, err := ReleaseCycleProcessingWithReason(opts, cycle, "cycle-failure-release"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if c := readItemContinuation(t, released); c != nil {
		t.Errorf("no manifest ⇒ no stamp, got %+v", c)
	}
}

func TestQuarantinePromotion_ShedsContinuationStamp(t *testing.T) {
	// failure_count 2 + ceiling 3 → the bump inside release reaches 3 and
	// quarantines. The quarantined item must NOT carry a stamp.
	opts, cycle, _ := stampFixture(t, true, 2)
	if _, err := ReleaseCycleProcessingWithQuarantine(opts, cycle, "cycle-failure-release", 3, false); err != nil {
		t.Fatalf("release: %v", err)
	}
	quarPath := filepath.Join(opts.ProjectRoot, ".evolve", "inbox", "quarantine", "task-a.json")
	if _, err := os.Stat(quarPath); err != nil {
		t.Fatalf("item must be quarantined at ceiling: %v", err)
	}
	if c := readItemContinuation(t, quarPath); c != nil {
		t.Errorf("quarantined item must shed the continuation stamp, got %+v", c)
	}
}

func TestResolveContinuation_FirstStampedClaimWins(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-95")
	if err := os.MkdirAll(procDir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(procDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a-plain.json", `{"id":"a"}`)
	write("b-stamped.json", `{"id":"b","continuation":{"snapshot_sha":"abc123","cycle":91}}`)
	write("c-stamped.json", `{"id":"c","continuation":{"snapshot_sha":"zzz999","cycle":90}}`)

	c := ResolveContinuation(Options{ProjectRoot: root}, 95)
	if c == nil || c.SnapshotSHA != "abc123" {
		t.Fatalf("want first stamped claim (b, abc123) deterministically, got %+v", c)
	}
	if got := ResolveContinuation(Options{ProjectRoot: root}, 96); got != nil {
		t.Errorf("no claims for cycle 96 ⇒ nil, got %+v", got)
	}
}
