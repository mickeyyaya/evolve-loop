package core

// legality_graph_test.go — PA-DDK DDK-5 (ADR-0060 §1a). The legality graph
// (`allowed`) is now config-driven via config.legal_successors, and the
// load-time validator is the relocated trust anchor that gates it. These tests
// load the real registry via the kerneltest fixture and reference phases through
// structural accessors — never hardcoded names — so renaming a phase needs no
// test rewrite.

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/kerneltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestLegalGraph_ConfigMatchesLiteral: the config-built legality graph from the
// reference registry is identical to the kernel's literal `allowed`. This is the
// DDK-5 equivalence oracle — config-driving the graph changes nothing for the
// shipped flow.
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

// TestWithLegalGraph_EmptyDegradesToLiteral: an empty config graph leaves the SM
// on its literal `allowed` (byte-identical bare SM / a registry omitting the map).
func TestWithLegalGraph_EmptyDegradesToLiteral(t *testing.T) {
	t.Parallel()
	literal := NewStateMachine().allowed
	sm := NewStateMachine().WithLegalGraph(legalGraphFrom(nil))
	if diff := diffGraph(literal, sm.allowed); diff != "" {
		t.Errorf("an empty legal graph must leave the literal graph unchanged:\n%s", diff)
	}
}

// TestValidateSafetyInvariants_ConfigGraphStrandsShip: a config graph that drops
// every edge INTO the ship terminal makes it unreachable; the validator — now
// quantifying over the CONFIG graph, not the literal — rejects it at load.
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

// TestOrchestrator_UnsafeLegalGraphFailsClosed: the validator is wired as a HARD
// gate — an orchestrator constructed with an unsafe legality graph refuses to run
// a cycle, returning ErrUnsafeConfig before any phase executes. This is the
// trust-anchor relocation made enforceable (DDK-1 landed it dark).
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
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if !errors.Is(err, ErrUnsafeConfig) {
		t.Fatalf("RunCycle must fail closed on an unsafe legality graph; got err=%v", err)
	}
	if len(res.PhasesRun) != 0 {
		t.Errorf("no phase may run under an unsafe config; ran %v", res.PhasesRun)
	}
}

// TestOrchestrator_SafeConfigRunsNormally: the guard does not false-positive — an
// orchestrator built with the real reference config runs a full cycle.
func TestOrchestrator_SafeConfigRunsNormally(t *testing.T) {
	ref := kerneltest.Load(t)
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil),
		WithRouting(ref.Config, router.StaticPreset{}),
		WithCatalog(ref.Catalog))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err != nil {
		t.Fatalf("the reference config is safe and must run; got err=%v", err)
	}
}

// --- graph test helpers (no hardcoded phase names) ---

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
