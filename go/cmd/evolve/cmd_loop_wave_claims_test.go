package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestWaveEngine_PassesTheLoopsGoalToTheClaimRelease(t *testing.T) {
	root := curationRoot(t, map[string]string{
		"processing/cycle-1836/held.json": `{"id":"held"}`,
		"a.json":                          `{"id":"a","weight":0.5,"files":["pkg/a.go"]}`,
		"b.json":                          `{"id":"b","weight":0.5,"files":["pkg/b.go"]}`,
	})
	evolveDir := filepath.Join(root, ".evolve")
	runDir := filepath.Join(evolveDir, "runs", "cycle-1836")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"cycle_id":1836,"phase":"tdd","goal_hash":"goal-wave-80","checkpoint":{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd"}}`
	if err := os.WriteFile(filepath.Join(runDir, "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	decision := `{"top_n":[{"id":"held","files":["pkg/held.go"]},{"id":"a","files":["pkg/a.go"]}]}`
	if err := os.WriteFile(filepath.Join(runDir, "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	storage := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 1836}}
	engine := newWaveEngine(loopConfig{ProjectRoot: root, EvolveDir: evolveDir, GoalHash: "goal-wave-81"}, storage, &warn, nilSignals)

	plan, _, err := engine.PlanFn(2)(context.Background(), 81)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plan), `"held"`) {
		t.Errorf("the prior decision in the last cycle's workspace keeps the released item: %s", plan)
	}

	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "held.json")); err != nil {
		t.Errorf("the loop's goal must release the paused claim at the first planning: %v\n%s", err, warn.String())
	}
	if !strings.Contains(warn.String(), "but the running loop has a newer goal") {
		t.Errorf("warn = %q, want the release line", warn.String())
	}
}
