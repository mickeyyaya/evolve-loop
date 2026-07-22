package core

// cyclerun_epilogue_test.go — cycle-1048 pins: every started cycle leaves a
// dossier + digest + coherent state on EVERY exit path. The abort path
// (`return cr.result, err` at loopAbort) skipped finalizeCycle, leaving a
// two-hour stale phase=retro record and a dossier-less, monitor-invisible
// failure.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func epilogueRun(t *testing.T, completedNormally bool) (*cycleRun, string) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-77")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cr := &cycleRun{
		o:     o,
		req:   CycleRequest{ProjectRoot: root, GoalHash: "goal-hash-77", Context: map[string]string{}},
		cycle: 77,
		cs:    CycleState{CycleID: 77, Phase: "retro", ActiveAgent: "retro", WorkspacePath: ws},
	}
	cr.cycleCompletedNormally = completedNormally
	return cr, root
}

func TestAbnormalEpilogue_WritesDossierDigestAndCoherentState(t *testing.T) {
	cr, root := epilogueRun(t, false)
	cr.abnormalEpilogue()

	dossier := filepath.Join(root, "knowledge-base", "cycles", "cycle-77.json")
	raw, err := os.ReadFile(dossier)
	if err != nil {
		t.Fatalf("abnormal exit must still write the dossier (monitors read them): %v", err)
	}
	if !strings.Contains(string(raw), "FAIL") || !strings.Contains(string(raw), "abnormal exit in phase retro") {
		t.Fatalf("dossier must record the abnormal outcome, got %s", raw)
	}
	if _, err := os.Stat(filepath.Join(cr.cs.WorkspacePath, "failure-digest.json")); err != nil {
		t.Fatal("abnormal exit must still produce the failure digest (breaker/disposition input)")
	}
	if cr.cs.Phase != "aborted" || cr.cs.ActiveAgent != "" {
		t.Fatalf("state must never claim a live phase for a dead cycle, got phase=%q agent=%q", cr.cs.Phase, cr.cs.ActiveAgent)
	}
}

func TestAbnormalEpilogue_NoopAfterNormalCloseout(t *testing.T) {
	cr, root := epilogueRun(t, true)
	cr.abnormalEpilogue()
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-77.json")); err == nil {
		t.Fatal("a normally-closed cycle must not get an epilogue dossier (no clobbering)")
	}
	if cr.cs.Phase != "retro" {
		t.Fatal("normal closeout state must be untouched by the epilogue")
	}
}
