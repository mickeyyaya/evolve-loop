package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestRealTreeRoutingOrder_AnchoredPhasesFollowTheirAnchors(t *testing.T) {
	t.Parallel()
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(registryPath); err != nil {
		t.Skipf("phase registry not present: %v", err)
	}
	cfg, _ := config.Load(registryPath, map[string]string{})
	if len(cfg.Order) == 0 {
		t.Skip("registry produced no order")
	}
	builtinCat, err := phasespec.Load(registryPath)
	if err != nil {
		t.Skipf("builtin catalog load failed: %v", err)
	}
	userSpecs, _ := discoverUserSpecsClamped(projectRoot, cmdutil.NewPromptsLoader(projectRoot))
	if len(userSpecs) == 0 {
		t.Skip("no user specs discovered")
	}
	phasespec.ApplyUserRouting(&cfg, userSpecs, builtinCat)

	idx := func(name string) int { return slices.Index(cfg.Order, name) }
	checked := 0
	for _, s := range userSpecs {
		if s.After == "" {
			continue
		}
		si, ai := idx(s.Name), idx(s.After)
		if si < 0 || ai < 0 {
			continue
		}
		checked++
		if si < ai {
			t.Errorf("phase %q (after: %q) placed at %d, BEFORE its anchor at %d — the cycle-1550 mis-slot", s.Name, s.After, si, ai)
		}
	}
	if checked == 0 {
		t.Skip("no anchored user specs routed")
	}

	bu := idx("build")
	for _, s := range userSpecs {
		if s.Stage != phasespec.StagePreBuild || s.After == "" {
			continue
		}
		si, ai := idx(s.Name), idx(s.After)
		if si >= 0 && ai >= 0 && bu >= 0 && (si < ai || si > bu) {
			t.Errorf("pre-build phase %q at %d must sit after its anchor %q (%d) and before build (%d); executing a red-first phase post-build reds the lane (cycle-1550)", s.Name, si, s.After, ai, bu)
		}
	}
}
