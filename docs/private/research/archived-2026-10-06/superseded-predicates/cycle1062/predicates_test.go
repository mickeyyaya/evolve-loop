//go:build acs

package cycle1062

import (
	"encoding/json"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC1062_008_ShadowStageWritesReportOnly(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	writeInboxItem(t, inboxDir, "recurring-defect", "pattern:flaky-tier", 0.80)
	stageEscalate(t, escDir, 1062, "pattern:flaky-tier", "recurring-defect", 4, 0.80)
	stageAutofile(t, escDir, 1062, "pattern:orphan", "orphan-defect", 3, 0.85)
	before := openItemCount(t, inboxDir)

	opts := applyOpts(root, 1062, true)
	res, err := recurrence.ApplyBoundary(opts)
	if err != nil {
		t.Fatalf("ApplyBoundary(shadow): %v", err)
	}
	if !res.Shadow {
		t.Errorf("result.Shadow = false for a shadow run; the report must record the stage")
	}
	if !acsassert.FileExists(t, opts.ReportPath) {
		t.Fatalf("shadow run wrote no report at %s; shadow must still emit the artifact", opts.ReportPath)
	}
	var report struct {
		Cycle  int  `json:"cycle"`
		Shadow bool `json:"shadow"`
	}
	raw, err := os.ReadFile(opts.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if report.Cycle != 1062 || !report.Shadow {
		t.Errorf("report = {cycle:%d shadow:%v}, want {1062 true}", report.Cycle, report.Shadow)
	}

	if n := openItemCount(t, inboxDir); n != before {
		t.Errorf("shadow run changed open inbox item count %d → %d; shadow must not mutate the inbox", before, n)
	}
	if w, _, _ := itemWeight(t, inboxDir, "recurring-defect"); w != 0.80 {
		t.Errorf("shadow run bumped a weight 0.80 → %v; shadow is report-only", w)
	}
	if _, _, ok := itemWeight(t, inboxDir, "orphan-defect"); ok {
		t.Errorf("shadow run FILED the autofile intent into the inbox; shadow is report-only")
	}
}

func TestC1062_009_AutofileGoesThroughRetrofile(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, "inbox")
	escDir := filepath.Join(root, "escalations")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	stageAutofile(t, escDir, 1062, "pattern:orphan", "orphan-defect", 3, 0.85)

	res, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Filed) != 1 {
		t.Fatalf("Filed = %v, want exactly 1 autofiled item", res.Filed)
	}

	wantPath := filepath.Join(inboxDir, "auto-retro-1062-orphan-defect.json")
	if !acsassert.FileExists(t, wantPath) {
		t.Fatalf("autofile did not emit %s; the autofile backend must be retrofile.FileActions, not a hand-rolled filer", wantPath)
	}
	raw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read filed item: %v", err)
	}
	var item struct {
		ID         string  `json:"id"`
		Weight     float64 `json:"weight"`
		Recurrence int     `json:"recurrence"`
		InjectedBy string  `json:"injected_by"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("filed item is not valid JSON: %v", err)
	}
	if item.ID != "orphan-defect" {
		t.Errorf("filed item id = %q, want \"orphan-defect\"", item.ID)
	}
	if item.InjectedBy != "retro-preventive-actions-autofiler" {
		t.Errorf("filed item injected_by = %q, want retrofile's own value \"retro-preventive-actions-autofiler\"", item.InjectedBy)
	}
	if item.Weight != 0.85 {
		t.Errorf("filed item weight = %v, want the staged 0.85 (WeightHint must reach retrofile)", item.Weight)
	}
	if item.Recurrence != 3 {
		t.Errorf("filed item recurrence = %d, want 3 (the recurrence count must reach the queue)", item.Recurrence)
	}

	again, err := recurrence.ApplyBoundary(applyOpts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary (re-apply): %v", err)
	}
	if len(again.Filed) != 0 {
		t.Errorf("re-apply filed %v again; autofile must happen exactly once while the item is open", again.Filed)
	}
}
