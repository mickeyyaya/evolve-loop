package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestPhaseCardsFromCatalog(t *testing.T) {
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "scout"},                     // → plan
		{Name: "build", WritesSource: true}, // → build
		{Name: "audit"},                     // → evaluate
		{Name: "ship"},                      // → control (omitted)
		{Name: "retro"},                     // → control (omitted)
	})
	if err != nil {
		t.Fatal(err)
	}
	cards := phaseCardsFromCatalog(cat)

	byName := map[string]router.PhaseCard{}
	for _, c := range cards {
		byName[c.Name] = c
	}
	if _, ok := byName["ship"]; ok {
		t.Error("control-archetype phase 'ship' must be omitted from the advisor catalog")
	}
	if _, ok := byName["retro"]; ok {
		t.Error("control-archetype phase 'retro' must be omitted")
	}
	if byName["scout"].Role != "plan" {
		t.Errorf("scout role = %q, want plan", byName["scout"].Role)
	}
	if byName["build"].Role != "build" || !byName["build"].WritesSource {
		t.Errorf("build card = %+v, want build/writes-source", byName["build"])
	}
	if byName["audit"].Role != "evaluate" {
		t.Errorf("audit role = %q, want evaluate", byName["audit"].Role)
	}
	if len(cards) != 3 {
		t.Errorf("expected 3 composable cards (control omitted), got %d", len(cards))
	}
}

func TestWriteCatalog(t *testing.T) {
	t.Run("empty catalog renders nothing", func(t *testing.T) {
		var b strings.Builder
		writeCatalog(&b, nil)
		if b.Len() != 0 {
			t.Errorf("empty catalog must render nothing, got %q", b.String())
		}
	})
	t.Run("cards render under a SELECT heading with role + writes-source", func(t *testing.T) {
		var b strings.Builder
		writeCatalog(&b, []router.PhaseCard{
			{Name: "scout", Role: "plan"},
			{Name: "build", Role: "build", WritesSource: true},
		})
		out := b.String()
		for _, want := range []string{"prefer these over minting", "scout [plan]", "build [build, writes-source]"} {
			if !strings.Contains(out, want) {
				t.Errorf("catalog output missing %q; got:\n%s", want, out)
			}
		}
	})
}
