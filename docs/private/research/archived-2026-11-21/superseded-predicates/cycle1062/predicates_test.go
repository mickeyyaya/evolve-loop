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
