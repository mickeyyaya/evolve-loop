// cmd_cycle_advisor_signals_test.go — ADR-0103 unit 04: the phase advisor's
// WARN reaches the --simulate root's console sink and the durable stream, and
// the production composition root hands its Signal Center to the advisor.
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type stubLauncher struct{}

func (stubLauncher) Launch(context.Context, advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	return advisor.LaunchResponse{Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}, nil
}

// Test 49 — the render twin: a capture-write fault under --simulate renders
// the module tag on the console and lands in the cycle workspace's stream.
func TestWireSimulateOrchestrator_AdvisorWarningRenders(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	d := wireSimulateOrchestrator(root, evolveDir, &console)
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(filepath.Join(ws, "advisor-prompt-plan.txt"), 0o755); err != nil { // a directory at the artifact path: a write fault
		t.Fatal(err)
	}
	a := advisor.New(stubLauncher{}, advisor.Identity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"},
		func(path string, data []byte) error { return os.WriteFile(path, data, 0o644) },
		advisor.WithSignals(func() *signalcenter.Center { return d.Signals }))
	if _, err := a.Plan(router.RouteInput{Cycle: 5, Current: "start", Workspace: ws, ProjectRoot: root}); err != nil {
		t.Fatalf("a capture fault never fails the plan: %v", err)
	}
	if out := console.String(); !strings.Contains(out, "[advisor]") || !strings.Contains(out, "ADVISOR_CAPTURE_WRITE_FAILED") {
		t.Fatalf("the console sink renders the unit's WARN under --simulate: %q", out)
	}
	if data, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 5), "signals.ndjson")); err != nil || !strings.Contains(string(data), `"code":"ADVISOR_CAPTURE_WRITE_FAILED"`) {
		t.Errorf("the cycle-stamped signal is durable in the cycle workspace: %v %s", err, data)
	}
}

// Test 50 — the production root's ONE construction of the advisor carries the
// Signal Center option (wiring is non-optional).
func TestPhaseAdvisorRoot_WiresTheSignalCenter(t *testing.T) {
	src, err := os.ReadFile("cmd_cycle.go")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`(?s)core\.NewPhaseAdvisor\(.*?\n\t\)`).FindString(string(src))
	if call == "" {
		t.Fatal("cmd_cycle.go must construct the advisor through core.NewPhaseAdvisor")
	}
	if !strings.Contains(call, "core.WithAdvisorSignals(signals)") {
		t.Fatalf("the composition root must hand its Signal Center to the advisor:\n%s", call)
	}
}
