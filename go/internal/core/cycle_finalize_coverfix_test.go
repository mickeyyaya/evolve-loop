package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClearCompletedCycleMarker_NilNowDefaultsToWallClock(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	csPath, _, _ := writeFinalizeFixture(t, evolveDir, 5, 5)

	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if err != nil || !cleared {
		t.Fatalf("cleared=%v err=%v, want cleared with default clock", cleared, err)
	}
	if _, statErr := os.Stat(csPath); !os.IsNotExist(statErr) {
		t.Errorf("cycle-state.json still present: %v", statErr)
	}
}

func TestClearCompletedCycleMarker_AbsentMarkerIsNotAnError(t *testing.T) {
	t.Parallel()
	cleared, err := ClearCompletedCycleMarker(t.TempDir(), FinalizeOptions{})
	if cleared || err != nil {
		t.Fatalf("cleared=%v err=%v, want false,nil", cleared, err)
	}
}

func TestClearCompletedCycleMarker_UnreadableMarkerFailsLoudly(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(evolveDir, CycleStateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err == nil {
		t.Fatalf("cleared=%v err=%v, want false and a read error", cleared, err)
	}
}

func TestClearCompletedCycleMarker_CorruptMarkerFailsLoudlyAndStays(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	csPath := filepath.Join(evolveDir, CycleStateFile)
	if err := os.WriteFile(csPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err == nil {
		t.Fatalf("cleared=%v err=%v, want false and a parse error", cleared, err)
	}
	if _, statErr := os.Stat(csPath); statErr != nil {
		t.Errorf("corrupt marker must be left in place: %v", statErr)
	}
}

func TestClearCompletedCycleMarker_MarkerWithoutCycleIDIsLeftAlone(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	csPath := filepath.Join(evolveDir, CycleStateFile)
	if err := os.WriteFile(csPath, []byte(`{"phase":"ship"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err != nil {
		t.Fatalf("cleared=%v err=%v, want false,nil", cleared, err)
	}
	if _, statErr := os.Stat(csPath); statErr != nil {
		t.Errorf("marker without cycle_id must stay: %v", statErr)
	}
}

func TestClearCompletedCycleMarker_MissingStateJSONNeverCountsAsCompleted(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	csPath, statePath, _ := writeFinalizeFixture(t, evolveDir, 3, 3)
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err != nil {
		t.Fatalf("cleared=%v err=%v, want false,nil (lastCycleNumber reads 0)", cleared, err)
	}
	if _, statErr := os.Stat(csPath); statErr != nil {
		t.Errorf("in-progress marker removed: %v", statErr)
	}
}

func TestClearCompletedCycleMarker_CorruptStateJSONFailsLoudly(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	csPath, statePath, _ := writeFinalizeFixture(t, evolveDir, 3, 3)
	if err := os.WriteFile(statePath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err == nil {
		t.Fatalf("cleared=%v err=%v, want false and an error", cleared, err)
	}
	if _, statErr := os.Stat(csPath); statErr != nil {
		t.Errorf("marker must survive an unreadable state.json: %v", statErr)
	}
}

func TestClearCompletedCycleMarker_UnreadableStateJSONFailsLoudly(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	_, statePath, _ := writeFinalizeFixture(t, evolveDir, 3, 3)
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statePath, 0o755); err != nil {
		t.Fatal(err)
	}
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{})
	if cleared || err == nil {
		t.Fatalf("cleared=%v err=%v, want false and a read error", cleared, err)
	}
}

func TestClearCompletedCycleMarker_RemoveFailureIsReported(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	evolveDir := t.TempDir()
	writeFinalizeFixture(t, evolveDir, 5, 5)
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{Now: time.Now})
	if cleared || err == nil {
		t.Fatalf("cleared=%v err=%v, want false and a remove error", cleared, err)
	}
}
