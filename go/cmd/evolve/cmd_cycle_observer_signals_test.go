package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/observer"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestObserverAdapterConstructionsAreWired(t *testing.T) {
	t.Parallel()
	callRE := regexp.MustCompile(`\bobserver\.NewCoreAdapter\(`)
	var unwired []string
	constructions := 0
	for _, f := range sharedModuleFiles(t) {
		calls := len(callRE.FindAll(f.Src, -1))
		constructions += calls
		if calls == 0 || strings.Count(string(f.Src), ".Signals = ") >= calls {
			continue
		}
		unwired = append(unwired, f.Rel)
	}
	if constructions == 0 {
		t.Fatal("the production root constructs the live observer adapter (cmd_cycle.go)")
	}
	if len(unwired) != 0 {
		t.Errorf("observer adapter built without its Signal Center accessor: %v", unwired)
	}
}

func TestWireSimulateOrchestrator_ObserverWarningRenders(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	ca := observer.NewCoreAdapter()
	ca.Signals = func() *signalcenter.Center { return d.Signals }
	fileAsWorkspace := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(fileAsWorkspace, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cancel := ca.Start(context.Background(), "build", core.PhaseRequest{Cycle: 3, Workspace: fileAsWorkspace})
	cancel()
	d.Signals.Flush()
	if out := console.String(); !strings.Contains(out, "[observer] observer.warning WARN OBSERVER_EVENTS_SINK_OPEN_FAILED cycle=3 phase=build") || !strings.Contains(out, "origin=CoreAdapter.Start") || !strings.Contains(out, "step=open_sink") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 3), "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"OBSERVER_EVENTS_SINK_OPEN_FAILED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}
