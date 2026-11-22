package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func shipWarning(module signalcenter.Module, code signalcenter.Code) signalcenter.Event {
	return signalcenter.Event{Cycle: 7, RunID: "run-7", Phase: "ship", Module: module, Origin: "test",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: "lease broken"}
}

func TestRootSignalCenter_ShipWindowBreaksReachTheProjectStream(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	signals := newRootSignalCenter(root, evolveDir, io.Discard)

	signals.Emit(shipWarning(signalcenter.ModuleShip, ship.CodeShipWindowLeaseBroken))
	signals.Emit(shipWarning(signalcenter.ModuleOrchestrator, core.CodeShipWindowLeaseBroken))
	signals.Emit(shipWarning(signalcenter.ModuleShip, "SHIP_WARNING_OTHER"))
	signals.Flush()

	project, err := os.ReadFile(filepath.Join(evolveDir, signalcenter.StreamFileName))
	if err != nil {
		t.Fatalf("the project stream was not written: %v", err)
	}
	for _, code := range []signalcenter.Code{ship.CodeShipWindowLeaseBroken, core.CodeShipWindowLeaseBroken} {
		if !strings.Contains(string(project), string(code)) {
			t.Errorf("%s is missing from .evolve/signals.ndjson, so the wave watch cannot see a break", code)
		}
	}
	if strings.Contains(string(project), "SHIP_WARNING_OTHER") {
		t.Error("an ordinary cycle-stamped ship warning reached the project stream")
	}
	run, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), signalcenter.StreamFileName))
	if err != nil || !strings.Contains(string(run), string(ship.CodeShipWindowLeaseBroken)) {
		t.Errorf("the run stream lost the break (err=%v)", err)
	}
}
