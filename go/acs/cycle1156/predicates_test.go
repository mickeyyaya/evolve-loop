//go:build acs

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

func newInbox(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	return root, inbox
}

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

func testOpts(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{
		ProjectRoot: root,
		Stderr:      stderr,
		IsLandedFn:  func(string) (bool, error) { return true, nil },
	}
}

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

func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int, error) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code, err
}

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

func TestC1156_006_committed_id_quarantines_at_ceiling(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	writeItem(t, procDir, "poison-item", 1)
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

func TestC1156_007_passing_cycle_promotes_exactly_committed_ids(t *testing.T) {
	root, inbox := newInbox(t)
	procDir := filepath.Join(inbox, "processing", "cycle-1156")
	writeItem(t, procDir, "shipped-a", 0)
	writeItem(t, inbox, "shipped-b", 0)
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
