package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRealTreeRoutingOrder_EveryPreBuildPhaseSitsBeforeBuild(t *testing.T) {
	t.Parallel()
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(registryPath); err != nil {
		t.Skipf("phase registry not present: %v", err)
	}
	cfg, _ := config.Load(registryPath)
	builtinCat, err := phasespec.Load(registryPath)
	if err != nil {
		t.Skipf("builtin catalog load failed: %v", err)
	}
	userSpecs, _ := discoverUserSpecsClamped(projectRoot, cmdutil.NewPromptsLoader(projectRoot))
	if len(userSpecs) == 0 {
		t.Skip("no user specs discovered")
	}
	phasespec.ApplyUserRouting(&cfg, userSpecs, builtinCat)

	build := slices.Index(cfg.Order, "build")
	if build < 0 {
		t.Fatalf("the routing order has no build phase: %v", cfg.Order)
	}
	routedPreBuild := 0
	for _, s := range userSpecs {
		if s.Stage != phasespec.StagePreBuild {
			continue
		}
		at := slices.Index(cfg.Order, s.Name)
		if at < 0 {
			t.Errorf("pre-build phase %q is declared but not routed — the floor refused it or it was never placed (order %v)", s.Name, cfg.Order)
			continue
		}
		routedPreBuild++
		if at > build {
			t.Errorf("pre-build phase %q at %d runs AFTER build at %d; a red-first Evaluate phase post-build reds the lane (cycle-1550)", s.Name, at, build)
		}
	}
	if routedPreBuild < 2 {
		t.Errorf("want at least bug-reproduction and fault-localization routed as pre-build, got %d", routedPreBuild)
	}
}

func TestRealTreeRoutingOrder_ThePinReadsTheStageFieldNotPhaseNames(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("routing_order_realtree_test.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, name := range []string{`"bug-reproduction"`, `"fault-localization"`} {
		if strings.Contains(src, name) {
			t.Errorf("routing_order_realtree_test.go still names the phase %s; the window pin must read the stage field", name)
		}
	}
	if !strings.Contains(src, "StagePreBuild") {
		t.Error("routing_order_realtree_test.go must pin the window through phasespec.StagePreBuild")
	}
}
