package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
)

func TestOrchestrator_WritesPhaseUsageSidecar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	workspace := cycleWorkspaceDir(root, res.Cycle)

	for _, next := range res.PhasesRun {
		phaseName := string(next)
		path := filepath.Join(workspace, fmt.Sprintf("%s-usage.json", phaseName))
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("%s sidecar must be written: %v", path, rerr)
		}

		var sidecar outcome.UsageSidecar
		if err := json.Unmarshal(data, &sidecar); err != nil {
			t.Fatalf("%s sidecar must be valid JSON: %v", path, err)
		}

		if sidecar.Phase != phaseName {
			t.Errorf("got phase %q, want %q", sidecar.Phase, phaseName)
		}
		if sidecar.Verdict != "PASS" {
			t.Errorf("got verdict %q, want PASS", sidecar.Verdict)
		}
		if sidecar.AttemptCount != 1 {
			t.Errorf("got attempt_count %d, want 1", sidecar.AttemptCount)
		}
	}
}
