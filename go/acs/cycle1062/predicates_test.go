//go:build acs

package cycle1062

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func writeInboxItem(t *testing.T, inboxDir, id, pattern string, weight float64) string {
	t.Helper()
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	path := filepath.Join(inboxDir, id+".json")
	body, err := json.MarshalIndent(map[string]any{
		"id":      id,
		"action":  "fix " + pattern,
		"weight":  weight,
		"pattern": pattern,
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode inbox item: %v", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write inbox item: %v", err)
	}
	return path
}

func claimInboxItem(t *testing.T, inboxDir, openPath, cycle string) string {
	t.Helper()
	destDir := filepath.Join(inboxDir, "processing", "cycle-"+cycle)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir processing: %v", err)
	}
	dest := filepath.Join(destDir, filepath.Base(openPath))
	if err := os.Rename(openPath, dest); err != nil {
		t.Fatalf("claim rename: %v", err)
	}
	return dest
}

func stageEscalate(t *testing.T, escDir string, cycle int, pattern, itemID string, count int, weight float64) {
	t.Helper()
	if _, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      cycle,
		Pattern:    pattern,
		ItemID:     itemID,
		Action:     "escalate",
		Route:      "queue",
		Recurrence: count,
		Weight:     weight,
	}); err != nil {
		t.Fatalf("StageIntent(escalate): %v", err)
	}
}

func stageAutofile(t *testing.T, escDir string, cycle int, pattern, itemID string, count int, weight float64) {
	t.Helper()
	if _, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      cycle,
		Pattern:    pattern,
		ItemID:     itemID,
		Action:     "autofile",
		Route:      "queue",
		Recurrence: count,
		Weight:     weight,
	}); err != nil {
		t.Fatalf("StageIntent(autofile): %v", err)
	}
}

func applyOpts(root string, cycle int, shadow bool) recurrence.ApplyOptions {
	return recurrence.ApplyOptions{
		InboxDir:        filepath.Join(root, "inbox"),
		EscalationsPath: dispositionrouter.PendingActionsPath(filepath.Join(root, "escalations")),
		ReportPath:      filepath.Join(root, "escalation-apply-report.json"),
		Cycle:           cycle,
		Shadow:          shadow,
		Policy:          recurrence.DefaultEscalationPolicy(),
		Now:             time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
}

func itemWeight(t *testing.T, inboxDir, id string) (weight float64, path string, ok bool) {
	t.Helper()
	_ = filepath.Walk(inboxDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".json") || ok {
			return nil //nolint:nilerr // absent tree is a legitimate "not found"
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var it struct {
			ID     string  `json:"id"`
			Weight float64 `json:"weight"`
		}
		if json.Unmarshal(raw, &it) != nil || it.ID != id {
			return nil
		}
		weight, path, ok = it.Weight, p, true
		return nil
	})
	return weight, path, ok
}

func openItemCount(t *testing.T, inboxDir string) int {
	t.Helper()
	entries, err := os.ReadDir(inboxDir)
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

func TestC1062_001_RouterFloorForcesConsoleForGuardAbort(t *testing.T) {
	d := dispositionrouter.Decide("guard-abort", 1, "queue")
	if d.Route != "console" {
		t.Errorf("Decide(guard-abort, 1, queue).Route = %q, want \"console\" (floor)", d.Route)
	}
	if !d.Forced {
		t.Errorf("Decide(guard-abort, 1, queue).Forced = false, want true (floor decisions are forced)")
	}
	if d.Reason == "" {
		t.Errorf("forced Decision carries an empty Reason; the floor must say why")
	}

	if q := dispositionrouter.Decide("verdict-fail", 1, "queue"); q.Route != "queue" || q.Forced {
		t.Errorf("Decide(verdict-fail, 1, queue) = {Route:%q Forced:%v}, want {queue false}", q.Route, q.Forced)
	}
}

func TestC1062_002_RouterFloorForcesConsoleAtRecurrenceThree(t *testing.T) {
	if d := dispositionrouter.Decide("verdict-fail", 3, "queue"); d.Route != "console" || !d.Forced {
		t.Errorf("Decide(verdict-fail, 3, queue) = {Route:%q Forced:%v}, want {console true}", d.Route, d.Forced)
	}
	if d := dispositionrouter.Decide("verdict-fail", 9, "queue"); d.Route != "console" || !d.Forced {
		t.Errorf("Decide(verdict-fail, 9, queue) = {Route:%q Forced:%v}, want {console true}", d.Route, d.Forced)
	}
	if d := dispositionrouter.Decide("verdict-fail", 2, "queue"); d.Route != "queue" || d.Forced {
		t.Errorf("Decide(verdict-fail, 2, queue) = {Route:%q Forced:%v}, want {queue false} (floor is >=3)", d.Route, d.Forced)
	}
}

func TestC1062_003_RouterLLMMayRaiseNeverLowerForcedRouting(t *testing.T) {
	raised := dispositionrouter.Decide("verdict-fail", 1, "console")
	if raised.Route != "console" {
		t.Errorf("advisory raise ignored: Decide(verdict-fail, 1, console).Route = %q, want \"console\"", raised.Route)
	}

	for _, tc := range []struct {
		preClass   string
		recurrence int
	}{
		{"guard-abort", 1},
		{"verdict-fail", 3},
	} {
		got := dispositionrouter.Decide(tc.preClass, tc.recurrence, "queue")
		if got.Route != "console" || !got.Forced {
			t.Errorf("advisory LOWERED a forced route: Decide(%s, %d, queue) = {Route:%q Forced:%v}, want {console true}",
				tc.preClass, tc.recurrence, got.Route, got.Forced)
		}
	}

	if d := dispositionrouter.Decide("verdict-fail", 1, ""); d.Route != "queue" {
		t.Errorf("Decide(verdict-fail, 1, \"\").Route = %q, want \"queue\" (empty advisory is a no-op)", d.Route)
	}
}

func TestC1062_004_StagedIntentNeverWritesInboxMidFlight(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	openPath := writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	before, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("read seeded item: %v", err)
	}
	beforeCount := openItemCount(t, inboxDir)

	staged, err := dispositionrouter.StageIntent(escDir, dispositionrouter.Intent{
		Cycle:      1062,
		Pattern:    "pattern:flaky-tier",
		ItemID:     "recurring-defect",
		Action:     "escalate",
		Route:      "queue",
		Recurrence: 4,
		Weight:     0.80,
	})
	if err != nil {
		t.Fatalf("StageIntent: %v (S3 must create .evolve/escalations/ on first run)", err)
	}
	if want := dispositionrouter.PendingActionsPath(escDir); staged != want {
		t.Errorf("StageIntent path = %q, want %q", staged, want)
	}

	raw, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("staged file not readable: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("staged file has %d JSONL lines, want 1", len(lines))
	}
	var got dispositionrouter.Intent
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("staged line is not valid JSON: %v", err)
	}
	if got.ItemID != "recurring-defect" || got.Action != "escalate" || got.Recurrence != 4 {
		t.Errorf("staged intent round-trip = %+v, want ItemID=recurring-defect Action=escalate Recurrence=4", got)
	}

	after, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatalf("seeded inbox item disappeared during staging: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("StageIntent MUTATED an inbox item (race with inboxmover.Claim); staging must never write the inbox")
	}
	if n := openItemCount(t, inboxDir); n != beforeCount {
		t.Errorf("open inbox item count %d → %d during staging; want unchanged", beforeCount, n)
	}
}

func TestC1062_005_ApplyEscalationIdempotentPerCycleStamp(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	stageEscalate(t, escDir, 1062, "pattern:flaky-tier", "recurring-defect", 4, 0.80)

	first, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (first): %v", err)
	}
	if len(first.Bumped) != 1 {
		t.Fatalf("first apply Bumped = %v, want exactly 1 item", first.Bumped)
	}
	afterFirst, _, ok := itemWeight(t, inboxDir, "recurring-defect")
	if !ok {
		t.Fatalf("item vanished after first apply")
	}
	if want := recurrence.DefaultEscalationPolicy().Target(0.80, 4); afterFirst != want {
		t.Errorf("weight after first apply = %v, want %v (min(cap, base+step*(count-1)))", afterFirst, want)
	}

	second, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (second): %v", err)
	}
	if len(second.Bumped) != 0 {
		t.Errorf("second apply in the SAME cycle bumped %v, want none (per-cycle stamp)", second.Bumped)
	}
	afterSecond, _, _ := itemWeight(t, inboxDir, "recurring-defect")
	if afterSecond != afterFirst {
		t.Errorf("weight drifted on re-apply: %v → %v, want stable", afterFirst, afterSecond)
	}
}

func TestC1062_006_ApplyEscalationNeverLowersWeight(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	const hot = 0.97
	writeInboxItem(t, inboxDir, "already-hot", "pattern:hot", hot)
	stageEscalate(t, escDir, 1062, "pattern:hot", "already-hot", 2, 0.50)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	got, _, ok := itemWeight(t, inboxDir, "already-hot")
	if !ok {
		t.Fatalf("item vanished during apply")
	}
	if got < hot {
		t.Errorf("apply LOWERED weight %v → %v; escalation must never lower (result: %+v)", hot, got, res)
	}
}

func TestC1062_007_PlanEscalationSkipsClaimedItems(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	openPath := writeInboxItem(t, inboxDir, "claimed-item", "pattern:claimed", 0.70)
	claimed := claimInboxItem(t, inboxDir, openPath, "1061")
	stageEscalate(t, escDir, 1062, "pattern:claimed", "claimed-item", 5, 0.70)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	for _, id := range res.Bumped {
		if id == "claimed-item" {
			t.Errorf("applier bumped a CLAIMED item (%s); claimed items are in flight and must be skipped", id)
		}
	}
	if n := openItemCount(t, inboxDir); n != 0 {
		t.Errorf("applier RESURRECTED %d claimed item(s) into the dispatchable inbox; want 0", n)
	}
	got, path, ok := itemWeight(t, inboxDir, "claimed-item")
	if !ok {
		t.Fatalf("claimed item disappeared entirely (was at %s)", claimed)
	}
	if got != 0.70 {
		t.Errorf("claimed item weight mutated 0.70 → %v (at %s); want untouched", got, path)
	}
	if len(res.Skipped) == 0 {
		t.Errorf("result.Skipped is empty; a skipped claimed item must be reported, not silently dropped")
	}
}

func TestC1062_010_LoopBoundaryWiringExecutes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "cmd", "evolve")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", "TestRunLoop_EscalatesAtIterationBoundary", "-v", pkgDir,
	)
	combined := stdout + stderr
	if err != nil && code == 0 {
		t.Fatalf("could not run the loop-boundary wiring test: %v", err)
	}
	if code != 0 {
		t.Fatalf("loop-boundary wiring test FAILED (exit %d):\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestRunLoop_EscalatesAtIterationBoundary") {
		t.Fatalf("TestRunLoop_EscalatesAtIterationBoundary did not run (no tests matched) — the cmd_loop boundary call site is unwired:\n%s", combined)
	}
	if !strings.Contains(combined, "--- PASS: TestRunLoop_EscalatesAtIterationBoundary") {
		t.Errorf("expected an explicit PASS line for TestRunLoop_EscalatesAtIterationBoundary:\n%s", combined)
	}
}
