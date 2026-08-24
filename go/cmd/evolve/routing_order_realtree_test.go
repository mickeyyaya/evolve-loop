package main

// Layer-N+1 wiring pin for the anchor-fixpoint splice (cycle-1550): the REAL
// production composition — config.Load(registry) order + discoverUserSpecsClamped
// (alphabetically sorted, clamped) + ApplyUserRouting — must place every
// anchored tracked phase AFTER its declared anchor. The unit fixpoint tests in
// internal/phasespec can pass while this path still mis-slots (different specs,
// different order source); this test runs the exact seam cmd_cycle.go runs.

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
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(projectRoot, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(registryPath); err != nil {
		t.Skipf("phase registry not present: %v", err)
	}
	cfg, _ := config.Load(registryPath, map[string]string{}) // tolerant posture, same seam as cmd_cycle (empty env: no operator overrides)
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
			continue // not routed (invalid / anchor outside order) — fallback path, warned elsewhere
		}
		checked++
		if si < ai {
			t.Errorf("phase %q (after: %q) placed at %d, BEFORE its anchor at %d — the cycle-1550 mis-slot", s.Name, s.After, si, ai)
		}
	}
	if checked == 0 {
		t.Skip("no anchored user specs routed")
	}

	// The incident phase specifically: a red-first Evaluate deliverable is only
	// coherent PRE-build. Pin it by name so a future registry/catalog reshuffle
	// that reintroduces the post-build slot fails here with the incident's name
	// on it, not just the generic anchor rule above.
	if bi, fi, bu := idx("bug-reproduction"), idx("fault-localization"), idx("build"); bi >= 0 && fi >= 0 && bu >= 0 {
		if bi < fi || bi > bu {
			t.Errorf("bug-reproduction at %d must sit after fault-localization (%d) and before build (%d); executing a red-first repro post-build reds the lane (cycle-1550)", bi, fi, bu)
		}
	}
}
