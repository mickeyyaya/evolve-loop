package core

import (
	"path/filepath"
	"testing"
)

// TestBackfillArtifactPath_AllPhases pins the phase→filename mapping that
// backfillArtifactPath must produce (cycle-187 AC-3/AC-4, Scout BA-1). The
// orchestrator passes this path to backfill.TryExtract; a wrong filename means
// a backfilled artifact lands where the next phase never reads it.
//
// RED at baseline for "retro" and "build-planner": the function's default
// branch yields "retro-report.md" / "build-planner-report.md", but the agents
// write "retrospective-report.md" / "build-plan.md". The other rows are
// regression guards so a future phase addition can't silently re-break the
// default-branch phases.
func TestBackfillArtifactPath_AllPhases(t *testing.T) {
	const ws = "/ws"
	cases := []struct {
		phase string
		want  string
	}{
		{"retro", "retrospective-report.md"}, // AC-3 — fixed mapping
		{"build-planner", "build-plan.md"},   // AC-4 — fixed mapping
		{"tdd", "test-report.md"},            // regression (existing case)
		{"intent", "intent.md"},              // regression (existing case)
		{"scout", "scout-report.md"},         // regression (default branch)
		{"build", "build-report.md"},         // regression (default branch)
		{"audit", "audit-report.md"},         // regression (default branch)
	}
	for _, tc := range cases {
		t.Run(tc.phase, func(t *testing.T) {
			got := backfillArtifactPath(ws, tc.phase)
			want := filepath.Join(ws, tc.want)
			if got != want {
				t.Errorf("backfillArtifactPath(%q)=%q, want %q", tc.phase, got, want)
			}
		})
	}
}
