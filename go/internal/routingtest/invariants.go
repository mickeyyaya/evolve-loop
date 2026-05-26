package routingtest

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// invariantFn asserts a kernel-floor property over one PureKernel decision.
type invariantFn func(t *testing.T, in router.RouteInput, proposal *router.Proposal, got router.RouterDecision)

// invariantChecks are the non-bypassable floor properties ("model proposes,
// kernel disposes") asserted across the adversarial cross-product.
var invariantChecks = map[string]invariantFn{
	// The keystone: a proposal — however adversarial — can NEVER change the
	// kernel's chosen NextPhase. applyProposal only records a clamp.
	"proposal-never-weakens": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		base := router.Route(in, nil)
		if got.NextPhase != base.NextPhase {
			t.Errorf("proposal changed NextPhase %q→%q (kernel floor must be proposal-invariant)", base.NextPhase, got.NextPhase)
		}
	},
	"mandatory-never-skipped": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		for _, m := range in.Cfg.Mandatory {
			if subset([]string{m}, got.SkipPhases) {
				t.Errorf("mandatory %q appears in SkipPhases %v", m, got.SkipPhases)
			}
		}
	},
	"no-ship-before-audit": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		if got.NextPhase == "ship" && !subset([]string{"audit"}, in.Completed) && in.Current != "audit" {
			t.Errorf("NextPhase=ship without audit completed or current; completed=%v current=%q", in.Completed, in.Current)
		}
	},
	"tdd-pin-nontrivial": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		size := in.Signals.CycleSize()
		if size != "" && size != "trivial" && subset([]string{"tdd"}, got.SkipPhases) {
			t.Errorf("tdd skipped on non-trivial cycle %q; skips=%v", size, got.SkipPhases)
		}
	},
	"insert-le-cap": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		if len(got.InsertPhases) > in.Cfg.MaxInsertions {
			t.Errorf("InsertPhases=%d exceeds cap %d: %v", len(got.InsertPhases), in.Cfg.MaxInsertions, got.InsertPhases)
		}
	},
	"budget-zero-no-content-insert": func(t *testing.T, in router.RouteInput, _ *router.Proposal, got router.RouterDecision) {
		if in.BudgetRemaining > 0 {
			return
		}
		for _, p := range got.InsertPhases {
			if _, isContent := in.Cfg.Triggers[p]; isContent {
				t.Errorf("content phase %q inserted with budget<=0", p)
			}
		}
	},
	"determinism": func(t *testing.T, in router.RouteInput, proposal *router.Proposal, _ router.RouterDecision) {
		a := router.Route(in, proposal)
		b := router.Route(in, proposal)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("Route nondeterministic:\n a=%+v\n b=%+v", a, b)
		}
	},
}

func assertInvariants(t *testing.T, s ScenarioSpec, in router.RouteInput, proposal *router.Proposal, got router.RouterDecision) {
	for _, name := range s.Expect.Invariants {
		fn, ok := invariantChecks[name]
		if !ok {
			t.Fatalf("routingtest: unknown invariant %q", name)
		}
		fn(t, in, proposal, got)
	}
}
