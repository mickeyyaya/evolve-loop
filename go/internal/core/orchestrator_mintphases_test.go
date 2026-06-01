package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// fakeMinter is a core.PhaseMinter: it normalizes the spec (forces Optional,
// like the real registrar) and returns a fakeRunner, or an error for names it
// is told to reject.
type fakeMinter struct {
	reject map[string]bool
}

func (m fakeMinter) Register(cfg phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error) {
	if m.reject[cfg.Name] {
		return phasespec.PhaseSpec{}, nil, errRejected
	}
	spec := cfg.Spec()
	spec.Optional = true
	return spec, &fakeRunner{name: spec.Name, verdict: VerdictPASS}, nil
}

var errRejected = errTest("minter rejected")

type errTest string

func (e errTest) Error() string { return string(e) }

// mintOrchestrator builds an orchestrator with the built-in spine only — NO
// minted phase in catalog/order/runners yet — plus a fake minter.
func mintOrchestrator(t *testing.T, minter PhaseMinter) *Orchestrator {
	t.Helper()
	cfg := config.RoutingConfig{
		Stage: config.StageEnforce,
		Order: []string{"scout", "build", "audit", "ship"},
	}
	o := NewOrchestrator(nil, nil, map[Phase]PhaseRunner{
		PhaseBuild: &fakeRunner{name: "build", verdict: VerdictPASS},
	}, WithCatalog(phasespec.Catalog{}), WithRouting(cfg, nil), WithRegistrar(minter))
	return o
}

func mintPlan(names ...string) *router.PhasePlan {
	var mint []phaseconfig.PhaseConfig
	for _, n := range names {
		mint = append(mint, phaseconfig.PhaseConfig{
			PhaseSpec: phasespec.PhaseSpec{Name: n, Optional: true, After: "build"},
			Prompt:    "inline persona for " + n,
		})
	}
	return &router.PhasePlan{MintPhases: mint}
}

// TestRegisterMintedPhases_MakesPhaseDispatchableAndRoutable is the slice-12
// acceptance: a minted phase, absent from the build-time catalog/order/runners,
// is registered at cycle start and becomes BOTH dispatchable (in runners) and
// routable (recognized by candidatePhase + a legal forward edge) — with no Go
// recompile, the same path a built-in or build-time user phase takes.
func TestRegisterMintedPhases_MakesPhaseDispatchableAndRoutable(t *testing.T) {
	o := mintOrchestrator(t, fakeMinter{})
	o.registerMintedPhases(mintPlan("minted-reviewer"))

	if _, ok := o.runners[Phase("minted-reviewer")]; !ok {
		t.Fatal("minted phase not registered in runners map (not dispatchable)")
	}
	if got := o.candidatePhase("minted-reviewer"); got != Phase("minted-reviewer") {
		t.Errorf("candidatePhase=%q, want minted-reviewer (not in catalog)", got)
	}
	if !o.transitionLegal(PhaseBuild, Phase("minted-reviewer")) {
		t.Error("build → minted-reviewer must be a legal forward edge (registration splices it into cfg.Order after build via ApplyUserRouting)")
	}
}

// TestRegisterMintedPhases_RejectedPhaseIsSkipped proves a registrar rejection
// (e.g. out-of-envelope) is a loud skip, not a registered dead phase.
func TestRegisterMintedPhases_RejectedPhaseIsSkipped(t *testing.T) {
	o := mintOrchestrator(t, fakeMinter{reject: map[string]bool{"bad-phase": true}})
	o.registerMintedPhases(mintPlan("bad-phase"))

	if _, ok := o.runners[Phase("bad-phase")]; ok {
		t.Error("a rejected minted phase must NOT be registered")
	}
	if got := o.candidatePhase("bad-phase"); got != "" {
		t.Errorf("rejected phase must not be routable; candidatePhase=%q", got)
	}
}

// TestRegisterMintedPhases_DoesNotClobberBuiltin proves a minted phase that
// collides with a built-in name never overwrites the built-in runner.
func TestRegisterMintedPhases_DoesNotClobberBuiltin(t *testing.T) {
	o := mintOrchestrator(t, fakeMinter{})
	before := o.runners[PhaseBuild]
	o.registerMintedPhases(mintPlan("build")) // collides with built-in
	if o.runners[PhaseBuild] != before {
		t.Error("minted phase must not clobber the built-in build runner")
	}
}

// TestRegisterMintedPhases_NilPlanIsNoop guards the common path: no minted
// phases (or no plan) must leave the orchestrator byte-identical.
func TestRegisterMintedPhases_NilPlanIsNoop(t *testing.T) {
	o := mintOrchestrator(t, fakeMinter{})
	n := len(o.runners)
	o.registerMintedPhases(nil)
	o.registerMintedPhases(&router.PhasePlan{})
	if len(o.runners) != n {
		t.Errorf("runners count changed on a no-mint plan: %d → %d", n, len(o.runners))
	}
}
