package routingtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// verdictsByPhase converts a phase-name→verdict map (the routingtest spec form)
// into the core.Phase-keyed map fixtures.BuildRunners expects.
func verdictsByPhase(verdicts map[string]string) map[core.Phase]string {
	if verdicts == nil {
		return nil
	}
	out := make(map[core.Phase]string, len(verdicts))
	for name, v := range verdicts {
		out[core.Phase(name)] = v
	}
	return out
}

// failedRecords converts FailedRecordSpec into core.FailedRecord with a
// non-expired RecordedAt so the retro failure-adapter sees them as active.
func failedRecords(specs []FailedRecordSpec) []core.FailedRecord {
	out := make([]core.FailedRecord, 0, len(specs))
	for i, s := range specs {
		out = append(out, core.FailedRecord{
			Cycle:          i + 1,
			Verdict:        s.Verdict,
			Classification: s.Classification,
			RecordedAt:     nonExpiredRecordedAt,
		})
	}
	return out
}

// seedWorkspace writes handoff files into the cycle workspace and returns it.
func seedWorkspace(t *testing.T, projectRoot string, cycle int, files map[string]string) string {
	t.Helper()
	ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return ws
}

// readRoutingDecisions parses every routing-decision-*.json in the workspace.
func readRoutingDecisions(t *testing.T, workspace string) []router.RouterDecision {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(workspace, "routing-decision-*.json"))
	out := make([]router.RouterDecision, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var d router.RouterDecision
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("unmarshal %s: %v", p, err)
		}
		out = append(out, d)
	}
	return out
}
