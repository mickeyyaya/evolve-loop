// cmd_ship_signal_center_test.go — ADR-0103 unit 07 wiring proofs: the ship
// phase built by the orchestrator root carries the root's Signal Center, the
// landing's WARN renders at the --simulate root, and the standalone
// `evolve ship` root builds the one sink topology for its own Center.
package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Test 40 (the critic's fold: orchDeps carries no runners map and
// core.Orchestrator exposes only HasRunner, so the wiring is pinned at the
// source) — the ONE ship.New(ship.Config{ literal of the production root
// passes the root's Center; the behavioural proof is ship's own
// TestShipOptions_ThreadsSignals.
func TestWireOrchestratorDeps_ShipPhaseCarriesTheRootSignalCenter(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	sites := regexp.MustCompile(`ship\.New\(ship\.Config\{[^\n]*`).FindAllString(string(src), -1)
	if len(sites) != 1 {
		t.Fatalf("cmd_cycle.go builds the ship phase at exactly one site, found %d: %q", len(sites), sites)
	}
	if !strings.Contains(sites[0], "Signals: signals") {
		t.Errorf("the production root must hand the ship phase its Signal Center (ADR-0103 unit 07): %s", sites[0])
	}
}

// Test 41 — the landing's module tag renders at the --simulate root: a
// landing built on the root's Center with a failing tracked-binary reset
// reaches the console sink and the durable cycle-workspace stream.
func TestWireSimulateOrchestrator_ShipWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	failingReset := func(_ context.Context, args []string, _, _ io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "checkout" {
			return 1, nil
		}
		return 0, nil
	}
	l := landing.New(failingReset, func() landing.Streams { return landing.Streams{Stdout: io.Discard, Stderr: io.Discard} },
		landing.WithRun(string(core.PhaseShip), 3, ""), landing.WithSignals(func() *signalcenter.Center { return d.Signals }))
	if err := l.Integrate(context.Background(), landing.Integration{Branch: "main", CycleBranch: "cycle-3-branch", Binary: "go/evolve"}); err != nil {
		t.Fatal(err)
	}
	if out := console.String(); !strings.Contains(out, "[ship] ship.warning WARN SHIP_LANDING_BINARY_RESET_FAILED cycle=3 phase=ship") || !strings.Contains(out, "origin=Landing.Integrate") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 3), "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"SHIP_LANDING_BINARY_RESET_FAILED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}

// Test 42 — the standalone `evolve ship` root builds its own Center through
// the ONE sink topology (newRootSignalCenter), threads it into Options and
// flushes it: a WARN emitted on that Center renders on the given stderr.
func TestCmdShip_BuildsTheRootSignalCenter(t *testing.T) {
	src, err := os.ReadFile("cmd_ship.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"newRootSignalCenter(", "Signals:", ".Flush()"} {
		if !strings.Contains(string(src), needle) {
			t.Errorf("cmd_ship.go must spell %q: the manual/release root wires the landing's warnings (ADR-0103 unit 07)", needle)
		}
	}
	root := t.TempDir()
	var stderr bytes.Buffer
	signals := shipRootSignals(root, &stderr)
	if _, err := os.Stat(filepath.Join(root, ".evolve", "signals.ndjson")); !os.IsNotExist(err) {
		t.Errorf("the durable sink opens on the FIRST event only — a green manual ship touches no file: %v", err)
	}
	signals.Emit(signalcenter.Event{Module: signalcenter.ModuleShip, Origin: "Test.shipRoot", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: landing.CodeHeadReadFailed, Reason: "ship root wiring proof"})
	signals.Flush()
	if !strings.Contains(stderr.String(), "[ship] ship.warning WARN SHIP_LANDING_HEAD_READ_FAILED") {
		t.Errorf("the ship root's WARN renders on the operator's stderr: %q", stderr.String())
	}
	if data, err := os.ReadFile(filepath.Join(root, ".evolve", "signals.ndjson")); err != nil || !strings.Contains(string(data), "ship root wiring proof") {
		t.Errorf("a cycle-less ship signal is durable under <evolveDir>/signals.ndjson: %v %s", err, data)
	}
}
