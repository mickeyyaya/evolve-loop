package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func routerQuotaWall() Wall {
	return Wall{}.Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"})
}

func cliHealthRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_health":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func unwritableHealthStore(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "cli-health.json"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunCycle_ABenchThatFailsAtTheRouterWallStillPausesAndNamesTheFamily(t *testing.T) {
	t.Parallel()
	root := cliHealthRoot(t)
	unwritableHealthStore(t, root)
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	walled := WalkError{Wall: routerQuotaWall(), Err: wrapTransient(85)}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}), WithPlanner(walledPlanner{err: walled}))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the quota pause even when the bench fails", err)
	}
	if !strings.Contains(err.Error(), "bench claude at the router quota wall") {
		t.Fatalf("err=%v, want the failed bench named", err)
	}
}

func TestBenchQuotaFamilies_AStoreThatCannotBeWrittenIsAnErrorThatNamesTheFamily(t *testing.T) {
	t.Parallel()
	root := cliHealthRoot(t)
	unwritableHealthStore(t, root)
	cr := &cycleRun{o: &Orchestrator{now: time.Now}, req: CycleRequest{ProjectRoot: root}}

	err := cr.benchQuotaFamilies(routerQuotaWall())

	if err == nil || !strings.Contains(err.Error(), "bench claude at the router quota wall") {
		t.Fatalf("err=%v, want the failed bench that names the family", err)
	}
}

func TestBenchQuotaFamilies_ALoggedOutEndIsNotBenchedAsAQuotaWall(t *testing.T) {
	t.Parallel()
	root := cliHealthRoot(t)
	cr := &cycleRun{o: &Orchestrator{now: time.Now}, req: CycleRequest{ProjectRoot: root}}
	wall := Wall{}.Observe(FamilyEnd{CLI: "agy-tmux", Pattern: "login_required"}).Observe(FamilyEnd{CLI: "claude-tmux", Pattern: "rate_limit"})

	if err := cr.benchQuotaFamilies(wall); err != nil {
		t.Fatalf("err=%v, want the quota family benched", err)
	}

	got := clihealth.NewStore(root, time.Now).Active()
	if _, agy := got["agy"]; agy || len(got) != 1 {
		t.Fatalf("benches = %+v, want only the quota family, not the logged-out one", got)
	}
}
