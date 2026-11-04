package core_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func digestClassOfConflictCycle(t *testing.T, debuggerWrites string, signals map[string]any) (string, []string) {
	t.Helper()
	f := newConflictFixture(t, debuggerWrites, signals)
	res, _ := f.o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "merge-rung0-goal",
		Context: map[string]string{"commit_message": "test commit"},
	})
	ws := t.TempDir()
	raw, err := json.Marshal(map[string]any{"schema_version": 1, "phase": "ship", "reasons": res.FailReasons})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := core.AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d.PreClass, res.FailReasons
}

func TestShipJourney_ARealFleetRebaseConflictCycleIsClassedAsAConflictNotUnknown(t *testing.T) {
	t.Parallel()

	class, reasons := digestClassOfConflictCycle(t, "lane line\n", nil)

	if class != dossier.PreClassShipRebaseConflict {
		t.Fatalf("digest class = %q for reasons %v, want %q", class, reasons, dossier.PreClassShipRebaseConflict)
	}
}

func TestShipJourney_ADebuggerThatEndsAFleetRebaseConflictIsClassedAsAConflict(t *testing.T) {
	t.Parallel()

	class, reasons := digestClassOfConflictCycle(t, "lane line\n", map[string]any{"debugger.action": "END"})

	if class != dossier.PreClassShipRebaseConflict {
		t.Fatalf("digest class = %q for reasons %v, want %q", class, reasons, dossier.PreClassShipRebaseConflict)
	}
}
