package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// TestReadUpstreamBuildPlan pins the dispatch-seam population rule for ADR-0050
// Phase 3.7: the build phase's upstream build-plan body is served via the
// envelope ONLY at advisory+ with the planner enabled (via WorkflowPolicy.PhaseEnables)
// and a readable file; every other case returns "".
func TestReadUpstreamBuildPlan(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-plan.md"), []byte("PLAN"), 0o644); err != nil {
		t.Fatal(err)
	}
	planner := map[string]string{"build-planner": "on"}

	wsEmpty := t.TempDir()
	if err := os.WriteFile(filepath.Join(wsEmpty, "build-plan.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name         string
		stage        config.Stage
		phase        Phase
		phaseEnables map[string]string
		ws           string
		want         string
	}{
		{"off → empty even with planner+file", config.StageOff, PhaseBuild, planner, ws, ""},
		{"shadow → empty", config.StageShadow, PhaseBuild, planner, ws, ""},
		{"advisory+build+planner+file → content", config.StageAdvisory, PhaseBuild, planner, ws, "PLAN"},
		{"enforce+build+planner+file → content", config.StageEnforce, PhaseBuild, planner, ws, "PLAN"},
		{"advisory but non-build phase → empty", config.StageAdvisory, PhaseScout, planner, ws, ""},
		{"advisory+build but planner off → empty", config.StageAdvisory, PhaseBuild, map[string]string{}, ws, ""},
		{"advisory+build+planner but no file → empty", config.StageAdvisory, PhaseBuild, planner, t.TempDir(), ""},
		{"advisory+build+planner+empty file → empty", config.StageAdvisory, PhaseBuild, planner, wsEmpty, ""},
		{"advisory+build+planner but empty workspace → empty", config.StageAdvisory, PhaseBuild, planner, "", ""},
	} {
		if got := readUpstreamBuildPlan(c.stage, c.phase, c.phaseEnables, c.ws); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
