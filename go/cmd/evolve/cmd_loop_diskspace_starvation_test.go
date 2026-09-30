package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func writeFloorPolicy(t *testing.T, evolveDir, minFreeGiB string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"preflight":{"min_free_gib":` + minFreeGiB + `}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func starvationCoordinator(t *testing.T) (*loopBatchCoordinator, string) {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: io.Discard}
	return b, evolveDir
}

func TestObserveWorkSupply_QuotaShrunkWaveThatMissesSizedWidthSelfFiles(t *testing.T) {
	b, evolveDir := starvationCoordinator(t)
	var tr fleet.StarvationTracker
	fc := policy.FleetConfig{Count: 3, StarvationK: 1, StarvationWeight: 0.9}
	b.observeWorkSupply(0, fc, policy.FleetConfig{Count: 2}, 1, &tr)
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "fleet-work-supply-starvation.json")); err != nil {
		t.Fatalf("1 of 2 sized lanes after a 3->2 quota shrink is starvation, no todo filed: %v", err)
	}
}

func TestObserveWorkSupply_QuotaShrunkWaveThatDeliversSizedWidthFilesNothing(t *testing.T) {
	b, evolveDir := starvationCoordinator(t)
	var tr fleet.StarvationTracker
	fc := policy.FleetConfig{Count: 3, StarvationK: 1, StarvationWeight: 0.9}
	b.observeWorkSupply(0, fc, policy.FleetConfig{Count: 2}, 2, &tr)
	if _, err := os.Stat(filepath.Join(evolveDir, "inbox", "fleet-work-supply-starvation.json")); err == nil {
		t.Fatal("2 of 2 sized lanes is not starvation, a todo was filed")
	}
}

func TestDispatchFleetIteration_DiskBelowFloorStopsBeforeLaunching(t *testing.T) {
	for _, scheduling := range []string{"wave", "pool"} {
		b, evolveDir := starvationCoordinator(t)
		writeFloorPolicy(t, evolveDir, "1000000000")
		var console bytes.Buffer
		b.stderr = &console
		b.deps.Signals = newRootSignalCenter(b.cfg.ProjectRoot, evolveDir, &console)
		b.deps.Storage = &fixtures.FakeStorage{}
		var tr fleet.StarvationTracker
		d := b.dispatchFleetIteration(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: scheduling}, "/nonexistent/evolve", &tr)
		b.deps.Signals.Flush()
		if d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode {
			t.Errorf("%s: decision %+v, want a system halt (rc=%d) before launch", scheduling, d, systemFailureHaltExitCode)
		}
		if !strings.Contains(console.String(), "evolve gc") {
			t.Errorf("%s: console must name the operator fix:\n%s", scheduling, console.String())
		}
	}
}

func TestDispatchFleetIteration_DiskAboveFloorDoesNotHalt(t *testing.T) {
	b, evolveDir := starvationCoordinator(t)
	writeFloorPolicy(t, evolveDir, "0.001")
	b.deps.Signals = newRootSignalCenter(b.cfg.ProjectRoot, evolveDir, io.Discard)
	b.deps.Storage = &fixtures.FakeStorage{}
	var tr fleet.StarvationTracker
	d := b.dispatchFleetIteration(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"}, "/nonexistent/evolve", &tr)
	if d.flow == batchReturn {
		t.Fatalf("ample disk must not halt the run: %+v", d)
	}
}

func TestDefaultLoopPreflight_DiskFloorFromPolicyHaltsBoot(t *testing.T) {
	for _, c := range []struct {
		floor string
		want  looppreflight.CheckLevel
	}{{"1000000000", looppreflight.LevelHalt}, {"0.001", looppreflight.LevelPass}} {
		root := t.TempDir()
		evolveDir := filepath.Join(root, ".evolve")
		writeFloorPolicy(t, evolveDir, c.floor)
		res := defaultLoopPreflight(loopConfig{ProjectRoot: root, EvolveDir: evolveDir, SkipPreflightBoot: true}, io.Discard)
		found := false
		for _, ch := range res.Checks {
			if ch.Name == "disk-space" {
				found = true
				if ch.Level != c.want {
					t.Errorf("floor %s: disk-space level=%s want %s", c.floor, ch.Level, c.want)
				}
			}
		}
		if !found {
			t.Errorf("floor %s: the boot preflight ran no disk-space check", c.floor)
		}
	}
}
