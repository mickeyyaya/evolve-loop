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
	callRE := regexp.MustCompile(`\bobserver\.NewCoreAdapter\(`)
	moduleRoot := filepath.Join("..", "..")
	var unwired []string
	constructions := 0
	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") && path != moduleRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		calls := len(callRE.FindAll(src, -1))
		constructions += calls
		if calls == 0 || strings.Count(string(src), ".Signals = ") >= calls {
			return nil
		}
		rel, _ := filepath.Rel(moduleRoot, path)
		unwired = append(unwired, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if constructions == 0 {
		t.Fatal("the production root constructs the live observer adapter (cmd_cycle.go)")
	}
	if len(unwired) != 0 {
		t.Errorf("observer adapter built without its Signal Center accessor: %v", unwired)
	}
}

// The workspace is a file, so the adapter's WARN fires. Flush before reading:
// an Emit that finds a drain in progress returns before delivery.
func TestWireSimulateOrchestrator_ObserverWarningRenders(t *testing.T) {
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
