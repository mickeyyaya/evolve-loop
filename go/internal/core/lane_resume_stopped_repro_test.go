package core

import "testing"

func TestFindLaneResume_AStoppedForegroundResumeKeepsTheLaneCheckpoint(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	run := quotaPausedRun(11, worktree, "item-a")
	run.reason = "operator-requested"
	seedLaneRun(t, evolveDir, run)

	if _, ok := FindLaneResume(t.TempDir(), evolveDir, []string{"item-a"}, ResumeOptions{}); !ok {
		t.Fatal("FindLaneResume lost the lane checkpoint after an operator-requested stop")
	}
	if got, _ := PausedLaneScopes(t.TempDir(), evolveDir, ResumeOptions{}); len(got) != 1 {
		t.Fatalf("PausedLaneScopes = %d entries, want 1", len(got))
	}
}
