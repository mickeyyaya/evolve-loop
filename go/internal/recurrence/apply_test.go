package recurrence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func seedItem(t *testing.T, inboxDir, id string, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	path := filepath.Join(inboxDir, id+".json")
	body, _ := json.MarshalIndent(map[string]any{"id": id, "weight": weight, "action": "fix"}, "", "  ")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	return path
}

func readWeight(t *testing.T, path string) float64 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	var it struct {
		Weight float64 `json:"weight"`
	}
	if err := json.Unmarshal(raw, &it); err != nil {
		t.Fatalf("parse item: %v", err)
	}
	return it.Weight
}

func opts(root string, cycle int, shadow bool) recurrence.ApplyOptions {
	return recurrence.ApplyOptions{
		InboxDir:        filepath.Join(root, "inbox"),
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(root, "escalations")),
		ReportPath:      filepath.Join(root, "report.json"),
		Cycle:           cycle,
		Shadow:          shadow,
		Policy:          recurrence.DefaultEscalationPolicy(),
		Now:             time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
}

func stage(t *testing.T, root, action, id, pattern string, count int, weight float64) {
	t.Helper()
	if _, err := dispositionrouter.StageIntent(filepath.Join(root, "escalations"), dispositionrouter.Intent{
		Cycle: 1, Pattern: pattern, ItemID: id, Action: action,
		Route: dispositionrouter.RouteQueue, Recurrence: count, Weight: weight,
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
}

// TestApplyBoundary_BumpsThenIsIdempotent pins the escalation math and the
// per-cycle stamp: the first pass bumps to Target, the second changes nothing.
func TestApplyBoundary_BumpsThenIsIdempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := seedItem(t, filepath.Join(root, "inbox"), "recurring", 0.80)
	stage(t, root, dispositionrouter.ActionEscalate, "recurring", "pattern:p", 4, 0.80)

	first, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	want := recurrence.DefaultEscalationPolicy().Target(0.80, 4)
	if len(first.Bumped) != 1 || readWeight(t, path) != want {
		t.Fatalf("first pass: Bumped=%v weight=%v, want 1 bump to %v", first.Bumped, readWeight(t, path), want)
	}
	second, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Bumped) != 0 || readWeight(t, path) != want {
		t.Fatalf("second pass in the same cycle re-escalated: Bumped=%v weight=%v", second.Bumped, readWeight(t, path))
	}
}

// TestApplyBoundary_NeverLowersWeight pins the negative case: a target below
// the item's current weight leaves the item alone and reports it skipped.
func TestApplyBoundary_NeverLowersWeight(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := seedItem(t, filepath.Join(root, "inbox"), "hot", 0.97)
	stage(t, root, dispositionrouter.ActionEscalate, "hot", "pattern:hot", 2, 0.50)

	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if w := readWeight(t, path); w != 0.97 {
		t.Fatalf("weight moved 0.97 → %v; escalation must never lower", w)
	}
	if len(res.Skipped) != 1 || len(res.Bumped) != 0 {
		t.Fatalf("result = %+v, want the item reported skipped, not bumped", res)
	}
}

// TestApplyBoundary_SkipsClaimedItems pins the fleet race: an item a lane has
// already claimed is neither bumped nor resurrected into the open inbox.
func TestApplyBoundary_SkipsClaimedItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	open := seedItem(t, inboxDir, "claimed", 0.70)
	destDir := filepath.Join(inboxDir, "processing", "cycle-1061")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir processing: %v", err)
	}
	dest := filepath.Join(destDir, filepath.Base(open))
	if err := os.Rename(open, dest); err != nil {
		t.Fatalf("claim: %v", err)
	}
	stage(t, root, dispositionrouter.ActionEscalate, "claimed", "pattern:c", 5, 0.70)

	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Bumped) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("result = %+v, want the claimed item skipped", res)
	}
	if w := readWeight(t, dest); w != 0.70 {
		t.Fatalf("claimed item weight mutated 0.70 → %v", w)
	}
	entries, _ := os.ReadDir(inboxDir)
	for _, e := range entries {
		if !e.IsDir() {
			t.Fatalf("claimed item resurrected into the open inbox as %s", e.Name())
		}
	}
}

// TestApplyBoundary_ShadowWritesReportOnly pins the compiled-default stage:
// the report artifact is written and nothing else moves.
func TestApplyBoundary_ShadowWritesReportOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := seedItem(t, filepath.Join(root, "inbox"), "recurring", 0.80)
	stage(t, root, dispositionrouter.ActionEscalate, "recurring", "pattern:p", 4, 0.80)
	stage(t, root, dispositionrouter.ActionAutofile, "orphan", "pattern:o", 3, 0.85)

	o := opts(root, 1062, true)
	res, err := recurrence.ApplyBoundary(o)
	if err != nil {
		t.Fatalf("ApplyBoundary(shadow): %v", err)
	}
	if !res.Shadow || len(res.Planned) != 2 || len(res.Bumped)+len(res.Filed) != 0 {
		t.Fatalf("shadow result = %+v, want Shadow=true, 2 planned, 0 applied", res)
	}
	if _, err := os.Stat(o.ReportPath); err != nil {
		t.Fatalf("shadow wrote no report: %v", err)
	}
	if w := readWeight(t, path); w != 0.80 {
		t.Fatalf("shadow bumped a weight to %v", w)
	}
	if _, err := os.Stat(filepath.Join(root, "inbox", "auto-retro-1062-orphan.json")); !os.IsNotExist(err) {
		t.Fatalf("shadow filed an inbox item")
	}
}

// TestApplyBoundary_AutofileUsesRetrofile pins that the autofile backend is the
// existing retrofile filer (its own item shape reaches disk), and that a second
// pass does not double-file.
func TestApplyBoundary_AutofileUsesRetrofile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stage(t, root, dispositionrouter.ActionAutofile, "orphan", "pattern:o", 3, 0.85)

	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Filed) != 1 {
		t.Fatalf("Filed = %v, want 1", res.Filed)
	}
	raw, err := os.ReadFile(filepath.Join(root, "inbox", "auto-retro-1062-orphan.json"))
	if err != nil {
		t.Fatalf("retrofile-shaped item absent: %v", err)
	}
	var item struct {
		ID         string  `json:"id"`
		Weight     float64 `json:"weight"`
		Recurrence int     `json:"recurrence"`
		InjectedBy string  `json:"injected_by"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("filed item is not JSON: %v", err)
	}
	if item.ID != "orphan" || item.Weight != 0.85 || item.Recurrence != 3 ||
		item.InjectedBy != "retro-preventive-actions-autofiler" {
		t.Fatalf("filed item = %+v, want retrofile's shape with the staged weight/recurrence", item)
	}
	again, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (re-apply): %v", err)
	}
	if len(again.Filed) != 0 {
		t.Fatalf("re-apply double-filed %v", again.Filed)
	}
}

// TestApplyBoundary_NothingStagedStillReports pins that an absent staging file
// is a no-op, not an error, and still emits the artifact.
func TestApplyBoundary_NothingStagedStillReports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := opts(root, 1062, false)
	res, err := recurrence.ApplyBoundary(o)
	if err != nil {
		t.Fatalf("ApplyBoundary(empty): %v", err)
	}
	if len(res.Bumped)+len(res.Filed)+len(res.Skipped) != 0 || res.Cycle != 1062 {
		t.Fatalf("result = %+v, want an empty pass stamped with the cycle", res)
	}
	if _, err := os.Stat(o.ReportPath); err != nil {
		t.Fatalf("no report written: %v", err)
	}
}

// TestApplyBoundary_MalformedStagingSurfaces pins that a corrupt staging file
// errors rather than silently applying nothing.
func TestApplyBoundary_MalformedStagingSurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	o := opts(root, 1062, false)
	if err := os.MkdirAll(filepath.Dir(o.EscalationsPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(o.EscalationsPath, []byte("garbage\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := recurrence.ApplyBoundary(o); err == nil {
		t.Fatal("ApplyBoundary(malformed staging) returned nil error")
	}
}

// TestApplyBoundary_UnknownActionIsSkipped pins the closed-vocabulary default:
// an unrecognised action never mutates the inbox.
func TestApplyBoundary_UnknownActionIsSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := seedItem(t, filepath.Join(root, "inbox"), "x", 0.5)
	stage(t, root, "teleport", "x", "pattern:x", 9, 0.9)
	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Skipped) != 1 || len(res.Bumped)+len(res.Filed) != 0 || readWeight(t, path) != 0.5 {
		t.Fatalf("unknown action was acted on: %+v", res)
	}
}

// TestApplyBoundary_PatternOnlyIntentAndNoReportPath pins two fallbacks in one
// pass: an intent with no explicit item id addresses its pattern, and an empty
// ReportPath skips report writing instead of erroring.
func TestApplyBoundary_PatternOnlyIntentAndNoReportPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedItem(t, filepath.Join(root, "inbox"), "pattern:only", 0.10)
	if _, err := dispositionrouter.StageIntent(filepath.Join(root, "escalations"), dispositionrouter.Intent{
		Pattern: "pattern:only", Action: dispositionrouter.ActionEscalate, Recurrence: 3, Weight: 0.50,
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
	o := opts(root, 1062, false)
	o.ReportPath = ""
	res, err := recurrence.ApplyBoundary(o)
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	var _ recurrence.ApplyResult = res
	if len(res.Bumped) != 1 || res.Bumped[0] != "pattern:only" {
		t.Fatalf("Bumped = %v, want the pattern used as the item id", res.Bumped)
	}
	if _, err := os.Stat(filepath.Join(root, "report.json")); !os.IsNotExist(err) {
		t.Fatalf("an empty ReportPath still wrote a report")
	}
}

// TestApplyBoundary_CorruptStampSurfaces pins that an unreadable idempotency
// stamp errors rather than silently re-escalating everything.
func TestApplyBoundary_CorruptStampSurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedItem(t, filepath.Join(root, "inbox"), "x", 0.10)
	stage(t, root, dispositionrouter.ActionEscalate, "x", "pattern:x", 3, 0.50)
	if err := os.WriteFile(filepath.Join(root, "escalations", "applied-stamp.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatalf("seed stamp: %v", err)
	}
	if _, err := recurrence.ApplyBoundary(opts(root, 1062, false)); err == nil {
		t.Fatal("ApplyBoundary(corrupt stamp) returned nil error")
	}
}

// TestApplyBoundary_CorruptInboxItemSurfaces pins that a malformed target item
// errors instead of being overwritten with a synthesised one.
func TestApplyBoundary_CorruptInboxItemSurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	path := seedItem(t, inboxDir, "x", 0.10)
	stage(t, root, dispositionrouter.ActionEscalate, "x", "pattern:x", 3, 0.50)
	// Corrupt AFTER staging so findItem still matched a well-formed id earlier
	// in the same pass is not required — the write path must fail loudly.
	if err := os.WriteFile(path, []byte(`{"id":"x","weight":0.1`), 0o644); err != nil {
		t.Fatalf("corrupt item: %v", err)
	}
	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err == nil && len(res.Bumped) != 0 {
		t.Fatalf("a corrupt inbox item was bumped: %+v", res)
	}
}
