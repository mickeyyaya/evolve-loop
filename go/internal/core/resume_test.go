package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeStateFile(t *testing.T, dir string, body any) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "cycle-state.json")
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestLoadResumeState_HappyPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	worktree := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}

	state := map[string]any{
		"cycle_id": 7,
		"phase":    "audit",
		"checkpoint": map[string]any{
			"enabled":               true,
			"reason":                "quota-likely",
			"savedAt":               "2026-05-23T10:00:00Z",
			"resumeFromPhase":       "audit",
			"worktreePath":          worktree,
			"gitHead":               "abc123",
			"completedPhases":       []string{"scout", "triage", "tdd", "build"},
			"costAtCheckpoint":      1.23,
			"autoResumeAttempts":    1,
			"autoResumeMaxAttempts": 3,
		},
	}
	writeStateFile(t, evolveDir, state)

	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{
		CurrentHead: func(_ string) (string, error) { return "abc123\n", nil },
		PathExists:  func(_ string) bool { return true },
	})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}
	if rp.CycleID != 7 || rp.Phase != "audit" || rp.GitHead != "abc123" {
		t.Errorf("got %+v, expected cycle=7 phase=audit head=abc123", rp)
	}
	if len(rp.CompletedPhases) != 4 || rp.CompletedPhases[0] != "scout" {
		t.Errorf("completed=%v", rp.CompletedPhases)
	}
	if rp.WorktreePath != worktree {
		t.Errorf("worktree=%q", rp.WorktreePath)
	}
	if rp.CostAtPause != 1.23 {
		t.Errorf("cost=%v", rp.CostAtPause)
	}
}

func TestLoadResumeState_MissingFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	_, err := LoadResumeState(context.Background(), tmp, filepath.Join(tmp, ".evolve"), ResumeOptions{})
	if err == nil || !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("got %v, want ErrNoCheckpoint", err)
	}
}

func TestLoadResumeState_NoCheckpointBlock(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{"cycle_id": 3, "phase": "scout"})
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err == nil || !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("got %v, want ErrNoCheckpoint", err)
	}
}

func TestLoadResumeState_CheckpointDisabled(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id":   3,
		"checkpoint": map[string]any{"enabled": false, "resumeFromPhase": "audit"},
	})
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err == nil || !errors.Is(err, ErrNoCheckpoint) {
		t.Errorf("got %v, want ErrNoCheckpoint", err)
	}
}

func TestLoadResumeState_StaleHead(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id": 3,
		"checkpoint": map[string]any{
			"enabled":         true,
			"resumeFromPhase": "audit",
			"gitHead":         "abc123",
		},
	})
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{
		CurrentHead: func(_ string) (string, error) { return "def456\n", nil },
		PathExists:  func(_ string) bool { return true },
	})
	if err == nil || !errors.Is(err, ErrStaleCheckpoint) {
		t.Errorf("got %v, want ErrStaleCheckpoint", err)
	}
	if !strings.Contains(err.Error(), "git HEAD moved") {
		t.Errorf("error message: %v", err)
	}
}

func TestLoadResumeState_AllowHeadMoved(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id": 3,
		"checkpoint": map[string]any{
			"enabled":         true,
			"resumeFromPhase": "audit",
			"gitHead":         "abc123",
		},
	})
	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{
		AllowHeadMoved: true,
		CurrentHead:    func(_ string) (string, error) { return "def456\n", nil },
		PathExists:     func(_ string) bool { return true },
	})
	if err != nil {
		t.Fatalf("AllowHeadMoved should succeed: %v", err)
	}
	if rp.Phase != "audit" {
		t.Errorf("phase=%q", rp.Phase)
	}
}

func TestLoadResumeState_WorktreeMissing(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id": 3,
		"checkpoint": map[string]any{
			"enabled":         true,
			"resumeFromPhase": "audit",
			"worktreePath":    "/tmp/nonexistent-worktree",
		},
	})
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{
		CurrentHead: func(_ string) (string, error) { return "", nil },
		PathExists:  func(_ string) bool { return false },
	})
	if err == nil || !errors.Is(err, ErrStaleCheckpoint) {
		t.Errorf("got %v, want ErrStaleCheckpoint", err)
	}
	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("error message: %v", err)
	}
}

func TestLoadResumeState_MissingResumeFromPhase(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id":   3,
		"checkpoint": map[string]any{"enabled": true},
	})
	_, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err == nil || !errors.Is(err, ErrStaleCheckpoint) {
		t.Errorf("got %v, want ErrStaleCheckpoint", err)
	}
}

func TestLoadResumeState_UnknownHeadSkipsValidation(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	writeStateFile(t, evolveDir, map[string]any{
		"cycle_id": 3,
		"checkpoint": map[string]any{
			"enabled":         true,
			"resumeFromPhase": "audit",
			"gitHead":         "unknown",
		},
	})
	// CurrentHead reports a different value, but checkpoint's gitHead is
	// "unknown" so validation must skip and not call CurrentHead at all.
	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{
		CurrentHead: func(_ string) (string, error) { return "any-head", nil },
		PathExists:  func(_ string) bool { return true },
	})
	if err != nil {
		t.Fatalf("expected success when checkpoint head=unknown: %v", err)
	}
	if rp.Phase != "audit" {
		t.Errorf("phase=%q", rp.Phase)
	}
}
