package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeProcItem drops a minimal item into processing/cycle-<cycle>/.
func writeProcItem(t *testing.T, inbox, cycle, id string) {
	t.Helper()
	dir := filepath.Join(inbox, "processing", "cycle-"+cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"id":"` + id + `","title":"` + id + `"}`)
	if err := os.WriteFile(filepath.Join(dir, id+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemFailureCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m.FailureCount
}

// TestReleaseWithQuarantine_BelowCeilingReleasesAndCounts — below the ceiling an
// item returns to the inbox root with an incremented durable failure_count.
func TestReleaseWithQuarantine_BelowCeilingReleasesAndCounts(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeProcItem(t, inbox, "5", "task-a")

	opts := Options{ProjectRoot: root}
	res, err := ReleaseCycleProcessingWithQuarantine(opts, 5, "cycle-failure-release", 2, false)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if res.Recovered != 1 {
		t.Fatalf("Recovered = %d; want 1", res.Recovered)
	}
	rootItem := filepath.Join(inbox, "task-a.json")
	if _, err := os.Stat(rootItem); err != nil {
		t.Fatalf("item not at inbox root: %v", err)
	}
	if got := itemFailureCount(t, rootItem); got != 1 {
		t.Errorf("failure_count = %d; want 1 after one failure below ceiling", got)
	}
	if _, err := os.Stat(filepath.Join(inbox, "quarantine", "task-a.json")); err == nil {
		t.Error("item was quarantined below the ceiling")
	}
}

// TestReleaseWithQuarantine_AtCeilingQuarantines — once the persisted count
// reaches the ceiling the item routes to quarantine/ and is gone from the root.
func TestReleaseWithQuarantine_AtCeilingQuarantines(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	opts := Options{ProjectRoot: root}

	// First failure (count→1): released to root, below ceiling 2.
	writeProcItem(t, inbox, "5", "poison")
	if _, err := ReleaseCycleProcessingWithQuarantine(opts, 5, "cycle-failure-release", 2, false); err != nil {
		t.Fatal(err)
	}
	// Re-claim for the next cycle: move root item into processing/cycle-6/.
	if err := os.MkdirAll(filepath.Join(inbox, "processing", "cycle-6"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(inbox, "poison.json"), filepath.Join(inbox, "processing", "cycle-6", "poison.json")); err != nil {
		t.Fatal(err)
	}
	// Second failure (count→2): hits ceiling, quarantines.
	if _, err := ReleaseCycleProcessingWithQuarantine(opts, 6, "cycle-failure-release", 2, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(inbox, "poison.json")); err == nil {
		t.Error("poison still at inbox root — should be quarantined at the ceiling")
	}
	qItem := filepath.Join(inbox, "quarantine", "poison.json")
	if _, err := os.Stat(qItem); err != nil {
		t.Fatalf("poison not in quarantine: %v", err)
	}
	if got := itemFailureCount(t, qItem); got != 2 {
		t.Errorf("quarantined failure_count = %d; want 2 (the diagnostic count)", got)
	}
}

// TestReleaseWithQuarantine_SystemLevelNeverQuarantines — AC4 at the wiring
// level: a system-level failure past the ceiling still releases to root.
func TestReleaseWithQuarantine_SystemLevelNeverQuarantines(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeProcItem(t, inbox, "5", "sys-task")
	// Pre-seed a high failure_count so only the systemLevel flag can suppress it.
	p := filepath.Join(inbox, "processing", "cycle-5", "sys-task.json")
	if err := os.WriteFile(p, []byte(`{"id":"sys-task","failure_count":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root}
	if _, err := ReleaseCycleProcessingWithQuarantine(opts, 5, "cycle-failure-release", 2, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "sys-task.json")); err != nil {
		t.Errorf("system-level failure was quarantined — S3 halt must take precedence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "quarantine", "sys-task.json")); err == nil {
		t.Error("system-level item landed in quarantine/")
	}
}

// TestReleaseFromQuarantine_RoundTrips — the operator escape hatch returns an
// item to the inbox root and resets its failure budget.
func TestReleaseFromQuarantine_RoundTrips(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	qPath := filepath.Join(inbox, "quarantine", "freed.json")
	if err := os.MkdirAll(filepath.Dir(qPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(qPath, []byte(`{"id":"freed","failure_count":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root}
	res, err := ReleaseFromQuarantine(opts, "freed")
	if err != nil {
		t.Fatalf("release-from-quarantine: %v", err)
	}
	if res.DestPath != filepath.Join(inbox, "freed.json") {
		t.Errorf("DestPath = %q; want inbox root", res.DestPath)
	}
	if _, err := os.Stat(qPath); err == nil {
		t.Error("item still in quarantine/ after release")
	}
	if got := itemFailureCount(t, res.DestPath); got != 0 {
		t.Errorf("failure_count = %d; want 0 (reset on release)", got)
	}
	// Missing id is a clean not-found, not a panic.
	if _, err := ReleaseFromQuarantine(opts, "nope"); err == nil {
		t.Error("expected ErrNotFound for absent id")
	}
}

// TestShouldQuarantine_NamesThePredicate (apicover): the exported S5 decision
// predicate, exercised over its whole contract — quarantine at/over the
// ceiling for TASK-level failures only; system-level failures NEVER
// quarantine (S3 halt precedence) regardless of count; zero/negative ceiling
// disables quarantine.
func TestShouldQuarantine_NamesThePredicate(t *testing.T) {
	cases := []struct {
		count, ceiling int
		system         bool
		want           bool
	}{
		{2, 3, false, false}, // below ceiling
		{3, 3, false, true},  // at ceiling
		{9, 3, false, true},  // over ceiling
		{9, 3, true, false},  // system-level: S3 precedence, never quarantine
		{9, 0, false, false}, // ceiling 0 = disabled
	}
	for _, tc := range cases {
		if got := ShouldQuarantine(tc.count, tc.ceiling, tc.system); got != tc.want {
			t.Errorf("ShouldQuarantine(%d,%d,%v)=%v, want %v", tc.count, tc.ceiling, tc.system, got, tc.want)
		}
	}
}
