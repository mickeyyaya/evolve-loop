package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// probeRunner is a fakeRunner that also answers the persona availability
// question — the shape every dispatchable phase runner has.
type probeRunner struct {
	*fakeRunner
	persona error
}

func (p *probeRunner) PersonaAvailable() error { return p.persona }

// TestAdvisorPlanInput_ListsPhasesWithMissingPersona — 2026-09-09 token-waste
// root cause #2: the advisor kept selecting optional phases whose persona doc
// does not exist; each selection cost a dispatch, a skip and (before the
// deterministic-learning fix) a retrospective agent. The orchestrator asks
// every OPTIONAL runner for its persona at plan time and hands the absent
// ones to the router as environmental context, so the advisor never sees them
// as selectable and the floor clamp drops them if proposed anyway. Mandatory
// and floor phases are not probed: their absence must stay a loud dispatch
// failure, never a silent exclusion.
func TestAdvisorPlanInput_ListsPhasesWithMissingPersona(t *testing.T) {
	runners := buildRunners(nil)
	runners[Phase("amplify-tests")] = &probeRunner{fakeRunner: &fakeRunner{name: "amplify-tests"},
		persona: fmt.Errorf("amplify-tests: load agent: %w", ErrAgentDocMissing)}
	runners[Phase("coverage-gate")] = &probeRunner{fakeRunner: &fakeRunner{name: "coverage-gate"}}
	// Guard rails: a NON-optional catalog phase and a catalog-Optional phase the
	// operator configured mandatory both have a missing persona — neither may
	// be excluded; their absence stays a loud dispatch failure.
	runners[PhaseBuild] = &probeRunner{fakeRunner: &fakeRunner{name: "build"},
		persona: fmt.Errorf("build: load agent: %w", ErrAgentDocMissing)}
	runners[Phase("mutation-gate")] = &probeRunner{fakeRunner: &fakeRunner{name: "mutation-gate"},
		persona: fmt.Errorf("mutation-gate: load agent: %w", ErrAgentDocMissing)}
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "build", Optional: false},
		{Name: "amplify-tests", Optional: true, After: "build"},
		{Name: "coverage-gate", Optional: true, After: "build"},
		{Name: "mutation-gate", Optional: true, After: "build"},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mandatory = append(cfg.Mandatory, "mutation-gate")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithCatalog(cat), WithRouting(cfg, router.StaticPreset{}))
	var _ PersonaProber = runners[PhaseBuild].(*probeRunner) // the interface the planner asks through
	in := o.advisorPlanInput(context.Background(), "scout", router.RoutingSignals{}, CycleRequest{ProjectRoot: t.TempDir()}, State{}, CycleState{}, 1, nil, nil)
	if want := []string{"amplify-tests"}; !reflect.DeepEqual(in.UnavailablePhases, want) {
		t.Fatalf("UnavailablePhases = %v, want %v (optional + missing only; build is non-optional and mutation-gate is configured-mandatory — both stay loud)", in.UnavailablePhases, want)
	}
	if !errors.Is(o.unavailablePhaseReason("amplify-tests"), ErrAgentDocMissing) {
		t.Fatalf("the exclusion must carry the dispatch sentinel as its reason")
	}
}

// TestWriteRoutingContext_UnavailablePhasesAreNamedNotOffered: the advisor
// prompt drops a persona-less phase from the selectable list and names it in
// its own section, so the advisor cannot propose it from memory.
func TestWriteRoutingContext_UnavailablePhasesAreNamedNotOffered(t *testing.T) {
	in := router.RouteInput{
		Cfg:               config.RoutingConfig{Triggers: map[string]config.RoutingBlock{"amplify-tests": {}, "coverage-gate": {}}},
		UnavailablePhases: []string{"amplify-tests"},
	}
	var b strings.Builder
	writeRoutingContext(&b, in)
	out := b.String()
	section := strings.Index(out, "## Unavailable phases")
	if section < 0 {
		t.Fatalf("prompt must name the unavailable phases:\n%s", out)
	}
	offered := out[:section]
	if strings.Contains(offered, "- amplify-tests") || !strings.Contains(offered, "- coverage-gate") {
		t.Fatalf("selectable list must omit the persona-less phase and keep the rest:\n%s", offered)
	}
	if !strings.Contains(out[section:], "- amplify-tests") {
		t.Fatalf("unavailable section must list the phase:\n%s", out[section:])
	}
}
