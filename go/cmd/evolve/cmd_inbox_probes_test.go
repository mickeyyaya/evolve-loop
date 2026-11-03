package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const probedItem = `{"id":"probed-pool","kind":"feature","priority_class":"performance","weight":0.6,"origin":"console",` +
	`"title":"The pool logs each backfill","files":["go/internal/fleet/pool.go"],"summary":"s","fix":"f",` +
	`"acceptance":["the pool logs a backfill"],"probes":[{"kind":"test_exists","target":"TestPool_LogsABackfill"}]}`

func writeTreeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCmd_InboxAdd_RefusesAnItemWhoseProbesAlreadyPassOnTheTree(t *testing.T) {
	root := emptyInboxRoot(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	writeTreeFile(t, root, "go/internal/fleet/pool.go", "package fleet\n")

	if rc, _, stderr := addItem(t, probedItem); rc != 0 {
		t.Fatalf("a probe that fails: rc = %d stderr = %q, want filed", rc, stderr)
	}
	writeTreeFile(t, root, "go/internal/fleet/pool_test.go", "package fleet\n\nfunc TestPool_LogsABackfill(t *testing.T) {}\n")
	done := strings.Replace(probedItem, `"id":"probed-pool"`, `"id":"probed-done"`, 1)

	rc, _, stderr := addItem(t, done)

	if rc != 1 || !strings.Contains(stderr, "its work is done") || !strings.Contains(stderr, "test_exists TestPool_LogsABackfill: pass") {
		t.Fatalf("rc = %d stderr = %q, want 1 naming the passing probe", rc, stderr)
	}
	if matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*-probed-done.json")); len(matches) != 0 {
		t.Errorf("the done item was filed: %v", matches)
	}
}

func TestCmd_InboxCurate_TheBoundaryConsumesAProbeItemAndProposesAQuarantine(t *testing.T) {
	t.Parallel()
	f := newCuratorFixture(t, "enforce")
	f.repo.Write(".evolve/inbox/2026-10-02T00-00-00Z-probed-pool.json", probedItem)
	f.repo.Write(".evolve/inbox/2026-10-03T00-00-00Z-dead-pool.json", strings.NewReplacer(`"id":"probed-pool"`, `"id":"dead-pool"`, "go/internal/fleet/pool.go", "go/internal/gone/pool.go").Replace(probedItem))
	f.repo.Write("go/internal/fleet/pool_test.go", "package fleet\n\nfunc TestPool_LogsABackfill(t *testing.T) {}\n")
	f.repo.Git("add", ".")
	f.repo.Git("commit", "-q", "-m", "probes")

	rc, stdout, stderr := f.curate(t, "", "--apply")

	if rc != 0 || !strings.Contains(stdout, "consume probed-pool (sweep, applied): every probe of the item passes on the tree") {
		t.Fatalf("rc = %d stdout = %q stderr = %q, want the probe consume applied", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, "quarantine dead-pool (sweep, judgment)") {
		t.Errorf("stdout = %q, want a judgment quarantine of dead-pool", stdout)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", "inbox", "consumed", "2026-10-02T00-00-00Z-probed-pool.json")); err != nil {
		t.Errorf("the probe item is not in consumed/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.root, ".evolve", "inbox", "2026-10-03T00-00-00Z-dead-pool.json")); err != nil {
		t.Errorf("the quarantine proposal moved the item: %v", err)
	}
}
