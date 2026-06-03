package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// loaderWithPersonas builds a prompts.Loader rooted at a temp dir containing
// agents/<name>.md for each given persona name.
func loaderWithPersonas(t *testing.T, names ...string) *prompts.Loader {
	t.Helper()
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		body := "---\nname: " + n + "\n---\n\n# " + n + "\npersona body\n"
		if err := os.WriteFile(filepath.Join(agentsDir, n+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return prompts.NewFromDir(dir)
}

func TestRegisterBuiltinSpecRunners(t *testing.T) {
	// Catalog: an Evaluate-archetype spec phase WITH a persona (should wire),
	// a Plan-archetype spec phase WITHOUT a persona (should WARN+skip), a Control
	// phase (should be excluded regardless of persona), and a non-llm phase.
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "tester", Kind: "llm"},              // Evaluate (inferred) + persona present
		{Name: "plan-review", Kind: "llm"},         // Plan (inferred) + NO persona
		{Name: "memo", Kind: "llm"},                // Control (inferred) → excluded
		{Name: "ship", Kind: "native"},             // non-llm → skipped
		{Name: "architecture-design", Kind: "llm"}, // Plan + persona present
	})
	if err != nil {
		t.Fatal(err)
	}
	// Personas exist for tester + architecture-design only (NOT plan-review).
	prm := loaderWithPersonas(t, "evolve-tester", "evolve-architecture-design", "evolve-memo")

	runners := map[core.Phase]core.PhaseRunner{} // empty: nothing hand-wired
	var warn strings.Builder
	registerBuiltinSpecRunners(runners, cat, prm, nil, &warn)

	if _, ok := runners[core.Phase("tester")]; !ok {
		t.Error("tester (Evaluate, kind:llm, persona present) must get a fallback runner")
	}
	if _, ok := runners[core.Phase("architecture-design")]; !ok {
		t.Error("architecture-design (Plan, kind:llm, persona present) must get a fallback runner")
	}
	if _, ok := runners[core.Phase("plan-review")]; ok {
		t.Error("plan-review has no persona — must NOT get a runner")
	}
	if !strings.Contains(warn.String(), "plan-review") {
		t.Errorf("expected a WARN naming plan-review; got %q", warn.String())
	}
	if _, ok := runners[core.Phase("memo")]; ok {
		t.Error("memo is Control-archetype — must be excluded even with a persona present")
	}
	if _, ok := runners[core.Phase("ship")]; ok {
		t.Error("ship is non-llm — must be skipped")
	}
}

func TestRegisterBuiltinSpecRunners_DoesNotOverrideExisting(t *testing.T) {
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "tester", Kind: "llm"}})
	prm := loaderWithPersonas(t, "evolve-tester")
	sentinel := stubRunner{}
	runners := map[core.Phase]core.PhaseRunner{core.Phase("tester"): sentinel}
	registerBuiltinSpecRunners(runners, cat, prm, nil, &strings.Builder{})
	if runners[core.Phase("tester")] != core.PhaseRunner(sentinel) {
		t.Error("fallback must NOT override a phase that already has a hand-wired runner")
	}
}

type stubRunner struct{}

func (stubRunner) Name() string { return "stub" }
func (stubRunner) Run(_ context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, nil
}
