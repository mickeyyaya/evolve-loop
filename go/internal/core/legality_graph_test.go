package core

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestLegalGraph_ConfigMatchesLiteral(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	if len(ref.Config.LegalSuccessors) == 0 {
		t.Fatal("reference registry must declare config.legal_successors (DDK-5)")
	}
	configured := legalGraphFrom(ref.Config.LegalSuccessors)
	literal := NewStateMachine().allowed
	if diff := diffGraph(literal, configured); diff != "" {
		t.Errorf("config-built legality graph must equal the literal graph:\n%s", diff)
	}
}

func TestWithLegalGraph_EmptyDegradesToLiteral(t *testing.T) {
	t.Parallel()
	literal := NewStateMachine().allowed
	sm := NewStateMachine().WithLegalGraph(legalGraphFrom(nil))
	if diff := diffGraph(literal, sm.allowed); diff != "" {
		t.Errorf("an empty legal graph must leave the literal graph unchanged:\n%s", diff)
	}
}

func TestValidateSafetyInvariants_ConfigGraphStrandsShip(t *testing.T) {
	t.Parallel()
	ref := kerneltest.Load(t)
	ship := ref.ShipTerminal()
	broken := cloneSuccessors(ref.Config.LegalSuccessors)
	for k := range broken {
		broken[k] = without(broken[k], ship)
	}
	cfg := ref.Config
	cfg.LegalSuccessors = broken
	sm := NewStateMachine().WithLegalGraph(legalGraphFrom(broken))
	if !containsSubstr(ValidateSafetyInvariants(sm, cfg, ref.Catalog), "unreachable") {
		t.Error("a config legality graph that strands the ship terminal must be rejected at load")
	}
}

func TestOrchestrator_UnsafeLegalGraphFailsClosed(t *testing.T) {
	ref := kerneltest.Load(t)
	ship := ref.ShipTerminal()
	broken := cloneSuccessors(ref.Config.LegalSuccessors)
	for k := range broken {
		broken[k] = without(broken[k], ship)
	}
	cfg := ref.Config
	cfg.LegalSuccessors = broken

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}),
		WithCatalog(ref.Catalog))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if !errors.Is(err, ErrUnsafeConfig) {
		t.Fatalf("RunCycle must fail closed on an unsafe legality graph; got err=%v", err)
	}
	if len(res.PhasesRun) != 0 {
		t.Errorf("no phase may run under an unsafe config; ran %v", res.PhasesRun)
	}
}

// SpineFloor is dialed to shadow here because this test's fake runners write
// no artifacts, which the spine floor would otherwise abort on — that would
// test the floor, not this validator. Only ValidateSafetyInvariants accepting
// the reference config is pinned here.
func TestOrchestrator_SafeConfigRunsNormally(t *testing.T) {
	ref := kerneltest.Load(t)
	cfg := ref.Config
	cfg.SpineFloor = config.StageShadow
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil),
		WithRouting(cfg, router.StaticPreset{}),
		WithCatalog(ref.Catalog))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("the reference config is safe and must run; got err=%v", err)
	}
}

func diffGraph(want, got map[Phase]map[Phase]bool) string {
	var b strings.Builder
	for from, tos := range want {
		for to := range tos {
			if !got[from][to] {
				b.WriteString("missing edge " + string(from) + "→" + string(to) + "\n")
			}
		}
	}
	for from, tos := range got {
		for to := range tos {
			if !want[from][to] {
				b.WriteString("extra edge " + string(from) + "→" + string(to) + "\n")
			}
		}
	}
	return b.String()
}

func cloneSuccessors(m map[string][]string) map[string][]string {
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func without(ss []string, dropName string) []string {
	drop := phaseFromRouter(dropName)
	var out []string
	for _, s := range ss {
		if phaseFromRouter(s) != drop {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
