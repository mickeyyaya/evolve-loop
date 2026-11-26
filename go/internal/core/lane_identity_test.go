package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func laneIdentityStateFile(t *testing.T, checkpoint map[string]any) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	checkpoint["enabled"] = true
	checkpoint["reason"] = "quota-likely"
	checkpoint["resumeFromPhase"] = "build"
	checkpoint["savedAt"] = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.Marshal(map[string]any{"cycle_id": 7, "phase": "build", "checkpoint": checkpoint})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, CycleStateFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func TestLaneIdentity_LoadResumeStateReadsTheIdentityOfALaneCheckpoint(t *testing.T) {
	root, evolveDir := laneIdentityStateFile(t, map[string]any{"poolId": 41, "laneCycle": 7, "scope": []string{"item-a", "item-b"}})

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}

	if rp.PoolID != 41 || rp.LaneCycle != 7 || !reflect.DeepEqual(rp.Scope, []string{"item-a", "item-b"}) {
		t.Fatalf("resume point identity = pool %d, lane cycle %d, scope %v; want pool 41, lane cycle 7, scope [item-a item-b]", rp.PoolID, rp.LaneCycle, rp.Scope)
	}
}

func TestLaneIdentity_LoadResumeStateReadsNoIdentityFromALegacyCheckpoint(t *testing.T) {
	root, evolveDir := laneIdentityStateFile(t, map[string]any{})

	rp, err := LoadResumeState(context.Background(), root, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}

	if rp.PoolID != 0 || rp.LaneCycle != 0 || rp.Scope != nil {
		t.Fatalf("a legacy checkpoint yields identity pool %d, lane cycle %d, scope %v; want none", rp.PoolID, rp.LaneCycle, rp.Scope)
	}
}
