package main

// cmd_loop_quota_wakeat_test.go — the CONSUMER half of the inbox defect
// `quota-pause-wakeat-unpopulated`, tested across the real seam: the production
// checkpointer (core.QuotaBoundaryCheckpointer, registered by
// internal/checkpoint's init and reached here because the evolve binary links it)
// writes the quota-likely block, and detectQuotaPause — the function whose output
// becomes the `QUOTA-PAUSE: … wake-at=%s source=%s` line — reads it back.
//
// Before the fix the two halves agreed on nothing useful: the writer left both
// fields empty and the reader printed `wake-at= source=unknown`, so the
// ScheduleWakeup delay arithmetic in skills/loop/SKILL.md could never run. A
// checkpoint-package unit test alone would not have caught that, since the
// defect only shows where the writer meets the reader.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestDetectQuotaPause_EmitsNonEmptyWakeAtAndSource — end-to-end: whatever the
// production checkpointer writes must give detectQuotaPause a parseable wake-at
// and a named source, because those two values ARE the marker line an operator
// model schedules from.
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
	// The checkpointer splices into an existing cycle-state.json (the orchestrator
	// always wrote one by the time a phase hits the quota wall).
	seed, err := json.Marshal(map[string]any{"cycle_id": 656, "phase": "build"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), seed, 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 30, 9, 0, 0, 0, time.Local)
	cs := core.CycleState{CycleID: 656, Phase: "build", WorkspacePath: workspace}
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
	if qp.Source == "" || qp.Source == "unknown" {
		t.Errorf("source = %q, want the estimator's named source", qp.Source)
	}
	if qp.Cycle != 656 {
		t.Errorf("cycle = %d, want 656", qp.Cycle)
	}
	// The value must be the ISO 8601 shape SKILL.md's delay computation parses.
	if _, err := time.Parse("2006-01-02T15:04:05-0700", qp.WakeAt); err != nil {
		if _, err2 := time.Parse(time.RFC3339, qp.WakeAt); err2 != nil {
			t.Errorf("wake-at %q is neither ISO-8601-with-offset nor RFC3339: %v / %v", qp.WakeAt, err, err2)
		}
	}
}

// TestDetectQuotaPause_EmptySourceReadsAsUnknown — the reader's half of the
// never-blank invariant. A legacy checkpoint written before the fix (or by any
// other producer) can still carry an explicitly-empty quotaResetSource; the
// reader must normalize that to "unknown" rather than printing `source=` and
// leaving an operator guessing whether the field is missing or meaningless.
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
