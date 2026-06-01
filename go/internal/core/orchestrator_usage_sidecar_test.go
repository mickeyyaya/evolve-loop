package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestOrchestrator_WritesPhaseUsageSidecar(t *testing.T) {
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	workspace := cycleWorkspaceDir(root, res.Cycle)

	// We want to verify that `<workspace>/<phase>-usage.json` is written for each phase run.
	for _, next := range res.PhasesRun {
		phaseName := string(next)
		path := filepath.Join(workspace, fmt.Sprintf("%s-usage.json", phaseName))
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("%s sidecar must be written: %v", path, rerr)
		}

		var sidecar phaseUsageSidecar
		if err := json.Unmarshal(data, &sidecar); err != nil {
			t.Fatalf("%s sidecar must be valid JSON: %v", path, err)
		}

		if sidecar.Phase != phaseName {
			t.Errorf("got phase %q, want %q", sidecar.Phase, phaseName)
		}
		if sidecar.Verdict != "PASS" {
			t.Errorf("got verdict %q, want PASS", sidecar.Verdict)
		}
		// cost_usd, duration_ms, attempt_count, verdict should be populated
		if sidecar.AttemptCount != 1 {
			t.Errorf("got attempt_count %d, want 1", sidecar.AttemptCount)
		}
	}
}
