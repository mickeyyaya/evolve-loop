package loopchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const rebuiltCommit = "0ddba11beef0"

func claim(pid int, commit string) Claim {
	return Claim{PID: pid, Commit: commit, At: fixedNow.Add(time.Minute)}
}

func TestTakeHandoff_TheReplacementImageResumesOnceAndNoOtherProcessInherits(t *testing.T) {
	f := newRefreshFixture(t)
	if !f.refresher(WithHandoff(Handoff{PID: 4242, WavesDone: 1})).Refresh(context.Background(), 7) {
		t.Fatalf("the refresh fires: %s", f.sig.console.String())
	}
	marker := filepath.Join(f.evolveDir, AttemptFile)
	raw, _ := os.ReadFile(marker)
	if !strings.Contains(string(raw), `"batch":7`) || !strings.Contains(string(raw), `"pid":4242`) || !strings.Contains(string(raw), `"waves_done":1`) {
		t.Fatalf("the armed marker carries the handoff in its own fields, beside the boundary: %s", raw)
	}

	if got, err := TakeHandoff(marker, claim(9999, rebuiltCommit)); got != 0 || err != nil {
		t.Errorf("a different pid never inherits: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, claim(4242, commit)); got != 0 || err != nil {
		t.Errorf("the image that armed it (its exec failed) never resumes itself: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 1 || err != nil {
		t.Fatalf("the replacement image resumes after the one completed wave, not at the boundary's batch number: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 0 || err != nil {
		t.Errorf("the handoff is consumed: %d %v", got, err)
	}
	consumed, _ := os.ReadFile(marker)
	if want := strings.ReplaceAll(golden(t, "marker.golden.json"), "{TS}", "2026-09-14T10:00:00Z"); string(consumed) != want {
		t.Errorf("consuming drops only the handoff; the breaker record stays: %s", consumed)
	}
}

func TestTakeHandoff_AStaleHandoffIsNotHonoured(t *testing.T) {
	for _, c := range []struct {
		name string
		age  time.Duration
		want int
	}{
		{"at the bound", handoffMaxAge, 2},
		{"past the bound", handoffMaxAge + time.Second, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), AttemptFile)
			writeJSON(t, marker, map[string]any{"running_commit": commit, "batch": 3, "timestamp": fixedNow.Format(time.RFC3339), "pid": 4242, "waves_done": 2})
			if got, err := TakeHandoff(marker, Claim{PID: 4242, Commit: rebuiltCommit, At: fixedNow.Add(c.age)}); got != c.want || err != nil {
				t.Errorf("age %v: got %d %v, want %d", c.age, got, err, c.want)
			}
		})
	}
	marker := filepath.Join(t.TempDir(), AttemptFile)
	writeJSON(t, marker, map[string]any{"running_commit": commit, "batch": 3, "timestamp": "not-a-time", "pid": 4242, "waves_done": 2})
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 0 || err != nil {
		t.Errorf("an undatable handoff is not honoured: %d %v", got, err)
	}
}

func TestTakeHandoff_AnAbsentOrCorruptMarkerStartsAtZeroAndAnUnwritableOneIsNotHonoured(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, AttemptFile)
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 0 || err != nil {
		t.Errorf("absent: %d %v", got, err)
	}
	if err := os.WriteFile(marker, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 0 || err != nil {
		t.Errorf("corrupt: %d %v", got, err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only file")
	}
	writeJSON(t, marker, map[string]any{"running_commit": commit, "batch": 3, "timestamp": fixedNow.Format(time.RFC3339), "pid": 4242, "waves_done": 2})
	if err := os.Chmod(marker, 0o444); err != nil {
		t.Fatal(err)
	}
	if got, err := TakeHandoff(marker, claim(4242, rebuiltCommit)); got != 0 || err == nil || !strings.Contains(err.Error(), "consume the boundary re-exec handoff") {
		t.Errorf("a handoff that cannot be consumed is not honoured, loudly: %d %v", got, err)
	}
}
