package core_test

// phaseoutputs_signal_test.go — the WIRING proof for the per-cycle
// phase-output survey signal. The first monitored wave (cycles 1452/1453)
// completed with ZERO phase-outputs-surveyed events because the emission
// lived on cmd_loop's single-loop path, which fleet lanes never traverse.
// This test drives a REAL RunCycle through the orchestrator and asserts the
// event reached the workspace's unified stream — so the emission can never
// again silently depend on how the cycle was dispatched.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

func TestRunCycle_EmitsPhaseOutputsSurveyToUnifiedStream(t *testing.T) {
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, _, _ := newTestOrchestrator(t, newRunners(nil))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*", "abnormal-events.jsonl"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no abnormal-events.jsonl in any cycle workspace (err=%v) — the survey signal never reached the unified stream", err)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var found *dispatchevents.Event
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var e dispatchevents.Event
		if json.Unmarshal([]byte(line), &e) == nil && e.EventType == dispatchevents.EventPhaseOutputsSurveyed {
			found = &e
			break
		}
	}
	if found == nil {
		t.Fatalf("stream has no %s event — cycles 1452/1453's silent non-reporting is back", dispatchevents.EventPhaseOutputsSurveyed)
	}
	if !strings.Contains(found.Details, "phase outputs:") || !strings.Contains(found.Details, "chain:") {
		t.Errorf("the event must carry the survey summary and chain state: %q", found.Details)
	}
}
