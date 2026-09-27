package core

import (
	"path/filepath"
	"testing"
)

// TestBackfillArtifactPath_AllPhases pins the phase→filename mapping
// backfillArtifactPath must produce. "retro" and "build-planner" override the
// default branch: the agents actually write retrospective-report.md and
// build-plan.md, not retro-report.md / build-planner-report.md.
func TestBackfillArtifactPath_AllPhases(t *testing.T) {
	t.Parallel()
	const ws = "/ws"
	cases := []struct {
		phase string
		want  string
	}{
		{"retro", "retrospective-report.md"},
		{"build-planner", "build-plan.md"},
		{"tdd", "test-report.md"},
		{"intent", "intent.md"},
		{"scout", "scout-report.md"},
		{"build", "build-report.md"},
		{"audit", "audit-report.md"},
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
