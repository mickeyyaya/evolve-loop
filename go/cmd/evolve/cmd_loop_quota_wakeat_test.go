package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestDetectQuotaPause_EmitsNonEmptyWakeAtAndSource(t *testing.T) {
	if core.QuotaBoundaryCheckpointer == nil {
		t.Fatal("core.QuotaBoundaryCheckpointer not registered — the evolve binary must link internal/checkpoint")
	}
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	workspace := filepath.Join(evolveDir, "runs", "cycle-656")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	// The checkpointer splices into an existing cycle-state.json.
	seed, err := json.Marshal(map[string]any{"cycle_id": 656, "phase": "build"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), seed, 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 30, 9, 0, 0, 0, time.Local)
	if _, err := clihealth.NewStore(projectRoot, func() time.Time { return now }).BenchWallUntil("claude", clihealth.Wall{Pattern: "exhausted", Reset: now.Add(3 * time.Hour)}); err != nil {
		t.Fatalf("bench a walled family: %v", err)
	}
	cs := core.CycleState{CycleID: 656, Phase: "build", WorkspacePath: workspace, QuotaWalkCLIs: []string{"claude-tmux"}}
	if err := core.QuotaBoundaryCheckpointer(cs, projectRoot, now); err != nil {
		t.Fatalf("quota-boundary checkpointer: %v", err)
	}

	qp, ok := detectQuotaPause(evolveDir)
	if !ok {
		t.Fatal("detectQuotaPause did not see the quota-likely checkpoint the seam just wrote")
	}
	if qp.WakeAt == "" {
		t.Error("wake-at is EMPTY — `QUOTA-PAUSE: … wake-at=` gives the resume scheduler nothing to parse (the defect)")
	}
	if qp.Source != "bench" {
		t.Errorf("source = %q, want bench: the walled family's reset is the estimator's named source", qp.Source)
	}
	if qp.Cycle != 656 {
		t.Errorf("cycle = %d, want 656", qp.Cycle)
	}
	// Either shape must parse: skills/loop/SKILL.md's delay computation accepts both.
	if _, err := time.Parse("2006-01-02T15:04:05-0700", qp.WakeAt); err != nil {
		if _, err2 := time.Parse(time.RFC3339, qp.WakeAt); err2 != nil {
			t.Errorf("wake-at %q is neither ISO-8601-with-offset nor RFC3339: %v / %v", qp.WakeAt, err, err2)
		}
	}
}

func TestDetectQuotaPause_EmptySourceReadsAsUnknown(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b, err := json.Marshal(map[string]any{
		"cycle_id": float64(9),
		"checkpoint": map[string]any{
			"enabled":          true,
			"reason":           "quota-likely",
			"quotaResetAt":     "2026-05-23T12:00:00Z",
			"quotaResetSource": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	qp, ok := detectQuotaPause(dir)
	if !ok {
		t.Fatal("detectQuotaPause returned !ok")
	}
	if qp.Source != "unknown" {
		t.Errorf("Source = %q, want \"unknown\" for an explicitly-empty quotaResetSource", qp.Source)
	}
}
