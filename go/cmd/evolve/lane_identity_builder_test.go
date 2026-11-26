package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaneIdentity_LoadBatchResumePointFallsBackToStateFileAndReportsMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	var lr loopResult
	rc := runResumeBatch(context.Background(), loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}, nil, newFakeLedger(), nil, nil, nil, &lr, &stdout, &stderr)
	if rc != 2 || !strings.Contains(stderr.String(), "evolve loop: resume:") {
		t.Fatalf("rc=%d stderr=%q, want missing checkpoint refusal", rc, stderr.String())
	}
}

func TestLaneIdentity_LegacyLaneCheckpointMalformedPolicyStageFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), []byte(`{"todo_ids":["item-a"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"integration":{"stage":"on"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rp := &core.ResumePoint{CycleID: 7, StatePath: filepath.Join(evolveDir, core.CycleStateFile)}
	err := legacyLaneCheckpointErr(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, rp, &bytes.Buffer{})
	if !errors.Is(err, errLegacyLaneCheckpointStage) {
		t.Fatalf("err=%v, want %v", err, errLegacyLaneCheckpointStage)
	}
}

func TestLaneIdentity_PreloadResumePointHandlesFreshAndMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got := preloadResumePoint(context.Background(), &loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}, &bytes.Buffer{}); got != nil {
		t.Fatalf("fresh preload = %+v, want nil", got)
	}
	if got := preloadResumePoint(context.Background(), &loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Resume: true}, &bytes.Buffer{}); got != nil {
		t.Fatalf("missing checkpoint preload = %+v, want nil", got)
	}
}

func TestLaneIdentity_RoutingRunIntegrationScopeFallsBackToEnvWhenUnset(t *testing.T) {
	t.Parallel()
	if got := (routingRun{}).integrationScope(); len(got) != 0 {
		t.Fatalf("integrationScope = %v, want empty when unset", got)
	}
}
