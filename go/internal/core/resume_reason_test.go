package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIsResumableReason_MatchesTheReasonsResumeDiscoveryAccepts(t *testing.T) {
	for reason, want := range map[string]bool{
		"quota-likely": true, "batch-cap-near": true, "operator-requested": true, "stall-inactivity": true,
		"phase-complete": false, "": false, "Quota-Likely": false,
	} {
		if got := IsResumableReason(reason); got != want {
			t.Errorf("IsResumableReason(%q) = %v, want %v", reason, got, want)
		}
	}
}

func TestCheckpointResumable_IsTheRuleLoopResumeApplies(t *testing.T) {
	dir := t.TempDir()
	worktree := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	head := func(string) (string, error) { return "abc123\n", nil }
	opts := ResumeOptions{CurrentHead: head}
	cases := []struct {
		name, body string
		wantErr    error
	}{
		{"resumable", `{"cycle_id":7,"phase":"tdd","checkpoint":{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd","gitHead":"abc123","worktreePath":"` + worktree + `"}}`, nil},
		{"head moved", `{"cycle_id":7,"phase":"tdd","checkpoint":{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd","gitHead":"def456"}}`, ErrStaleCheckpoint},
		{"no resumeFromPhase", `{"cycle_id":7,"phase":"tdd","checkpoint":{"enabled":true,"reason":"quota-likely"}}`, ErrStaleCheckpoint},
		{"worktree gone", `{"cycle_id":7,"phase":"tdd","checkpoint":{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd","worktreePath":"` + filepath.Join(dir, "gone") + `"}}`, ErrStaleCheckpoint},
		{"disabled", `{"cycle_id":7,"phase":"tdd","checkpoint":{"enabled":false,"resumeFromPhase":"tdd"}}`, ErrNoCheckpoint},
		{"cycle ended", `{"cycle_id":7,"phase":"end","checkpoint":{"enabled":true,"resumeFromPhase":"tdd"}}`, ErrNoCheckpoint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rp, err := CheckpointResumable(write(tc.name+".json", tc.body), dir, opts)
			if tc.wantErr == nil {
				if err != nil || rp == nil || rp.CycleID != 7 || rp.Reason != "quota-likely" {
					t.Errorf("CheckpointResumable = %+v, %v; want cycle 7 resumable", rp, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
	if _, err := CheckpointResumable(filepath.Join(dir, "absent.json"), dir, ResumeOptions{}); !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("an absent state with default options: err = %v, want ErrNoCheckpoint", err)
	}
}
