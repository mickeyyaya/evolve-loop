package loopchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rebuiltCommit = "0ddba11beef0"

func TestTakeHandoff_TheReplacementImageResumesOnceAndNoOtherProcessInherits(t *testing.T) {
	f := newRefreshFixture(t)
	if !f.refresher(WithHandoff(4242)).Refresh(context.Background(), 2) {
		t.Fatalf("the refresh fires: %s", f.sig.console.String())
	}
	marker := filepath.Join(f.evolveDir, AttemptFile)
	raw, _ := os.ReadFile(marker)
	if !strings.Contains(string(raw), `"pid":4242`) || !strings.Contains(string(raw), `"batch":2`) {
		t.Fatalf("the armed marker carries the handoff: %s", raw)
	}

	if got, err := TakeHandoff(marker, 9999, rebuiltCommit); got != 0 || err != nil {
		t.Errorf("a different pid never inherits: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, 4242, commit); got != 0 || err != nil {
		t.Errorf("the image that armed it (its exec failed) never resumes itself: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, 4242, rebuiltCommit); got != 1 || err != nil {
		t.Fatalf("the replacement image resumes after the one completed iteration: %d %v", got, err)
	}
	if got, err := TakeHandoff(marker, 4242, rebuiltCommit); got != 0 || err != nil {
		t.Errorf("the handoff is consumed: a later batch in the same process starts at 0: %d %v", got, err)
	}
	consumed, _ := os.ReadFile(marker)
	want := strings.NewReplacer(`"batch":7`, `"batch":2`, "{TS}", "2026-09-14T10:00:00Z").Replace(golden(t, "marker.golden.json"))
	if string(consumed) != want {
		t.Errorf("consuming drops only the pid; the breaker record stays: %s", consumed)
	}
}

func TestTakeHandoff_AnAbsentOrCorruptMarkerStartsAtZeroAndAnUnwritableOneIsNotHonoured(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, AttemptFile)
	if got, err := TakeHandoff(marker, 4242, rebuiltCommit); got != 0 || err != nil {
		t.Errorf("absent: %d %v", got, err)
	}
	if err := os.WriteFile(marker, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := TakeHandoff(marker, 4242, rebuiltCommit); got != 0 || err != nil {
		t.Errorf("corrupt: %d %v", got, err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only file")
	}
	if err := os.WriteFile(marker, []byte(`{"running_commit":"`+commit+`","batch":3,"timestamp":"t","pid":4242}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(marker, 0o444); err != nil {
		t.Fatal(err)
	}
	if got, err := TakeHandoff(marker, 4242, rebuiltCommit); got != 0 || err == nil || !strings.Contains(err.Error(), "consume the boundary re-exec handoff") {
		t.Errorf("a handoff that cannot be consumed is not honoured, loudly: %d %v", got, err)
	}
}
