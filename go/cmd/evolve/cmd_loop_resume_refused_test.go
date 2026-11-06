package main

import (
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRefusedResumeLines_RenderTheWallOfTheCheckpoint(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		cp   checkpoint.Checkpoint
		want []string
	}{
		{
			"a wall of one login",
			checkpoint.Checkpoint{QuotaWall: []core.FamilyEnd{{CLI: "claude", Pattern: "auth_recheck"}}},
			[]string{"LOGIN-REQUIRED: cycle=7 claude needs a login. Next: open claude and run /login. Then evolve loop --resume."},
		},
		{
			"an older checkpoint with no wall",
			checkpoint.Checkpoint{},
			[]string{"WALL-PAUSE: cycle=7 no cause was recorded for this pause. Next: evolve loop --resume."},
		},
		{
			"an older checkpoint with a live wake time keeps its timed marker",
			checkpoint.Checkpoint{QuotaResetAt: "2026-11-06T10:30:00Z", QuotaResetSource: "bench", AutoResumeMaxAttempts: 3},
			[]string{"QUOTA-PAUSE: cycle=7 wake-at=2026-11-06T10:30:00Z source=bench attempts=0/3 resume-in=1860s (no cause was recorded for this pause)", "[loop]   next: evolve loop --resume."},
		},
		{
			"rows with no quota and no login keep the wake time",
			checkpoint.Checkpoint{QuotaResetAt: "2026-11-06T10:30:00Z", QuotaResetSource: "bench", AutoResumeMaxAttempts: 3, QuotaWall: []core.FamilyEnd{{CLI: "claude", Pattern: "bridge_crash"}}},
			[]string{"WALL-PAUSE: cycle=7 wake-at=2026-11-06T10:30:00Z source=bench attempts=0/3 resume-in=1860s claude ended on bridge_crash. Next: evolve loop --resume."},
		},
		{
			"a spent budget with no wall",
			checkpoint.Checkpoint{AutoResumeAttempts: 3, AutoResumeMaxAttempts: 3, OperatorAction: "the auto-resume budget is spent: no cause was recorded for this pause. Next: evolve loop --resume."},
			[]string{"WALL-PAUSE: cycle=7 attempts=3/3 auto-resume=off: the auto-resume budget is spent: no cause was recorded for this pause. Next: evolve loop --resume."},
		},
	}
	for _, c := range cases {
		if got := refusedResumeLines(7, c.cp, t.TempDir(), now); !slices.Equal(got, c.want) {
			t.Errorf("%s: lines = %q, want %q", c.name, got, c.want)
		}
	}
}
