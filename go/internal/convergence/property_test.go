package convergence

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	propertyInputs = 3000
	propertySeed   = 1947
)

var (
	propertyComponents = []string{"alpha", "beta", "gamma"}
	propertyLoops      = []Loop{LoopCodeReview, LoopConsoleLane, LoopAuditRepair, LoopExplanationReauthor, LoopCycle, LoopInboxItem, LoopShipRecovery}
	propertySeverities = []Severity{SeverityCritical, SeverityHigh, SeverityHigh, SeverityMedium, SeverityMedium, SeverityMedium, SeverityLow, SeverityLow, SeverityInfo}
	propertyStatuses   = []Status{StatusOpen, StatusOpen, StatusOpen, StatusFixed, StatusFixed, StatusDisputed}
)

type sketch struct {
	id        string
	severity  Severity
	component string
}

func pick[T any](r *rand.Rand, from []T) T { return from[r.IntN(len(from))] }

func randomSketches(r *rand.Rand) []sketch {
	out := make([]sketch, 4+r.IntN(8))
	for i := range out {
		out[i] = sketch{id: fmt.Sprintf("f%d", i), severity: pick(r, propertySeverities), component: pick(r, propertyComponents)}
	}
	return out
}

func residueStatus(index int, s sketch) Status {
	switch {
	case index == 0 && s.severity == SeverityCritical:
		return StatusOpen
	case s.severity == SeverityCritical || s.severity == SeverityHigh:
		return StatusFixed
	}
	return StatusOpen
}

func randomRound(r *rand.Rand, index int, sketches []sketch, residue bool) Judgment {
	j := Judgment{Index: index}
	for _, s := range sketches {
		if r.IntN(3) == 0 || (residue && index == 0 && s.severity != SeverityCritical) {
			continue
		}
		status := pick(r, propertyStatuses)
		if residue {
			status = residueStatus(index, s)
		}
		j.Findings = append(j.Findings, Finding{ID: s.id, Severity: s.severity, Component: s.component, Status: status})
	}
	return j
}

func randomInput(r *rand.Rand, residue bool) Input {
	sketches := randomSketches(r)
	if residue {
		sketches[0].severity = SeverityCritical
	}
	rounds := make([]Judgment, 1+r.IntN(5))
	for i := range rounds {
		rounds[i] = randomRound(r, i, sketches, residue)
	}
	cfg := policy.Policy{}.ConvergenceConfig()
	cfg.MaxFixRounds = 2 + r.IntN(4)
	cfg.MaxBackwardEdges = 2 + r.IntN(3)
	components := map[string]ComponentFacts{}
	for _, c := range propertyComponents {
		facts := ComponentFacts{Separable: r.IntN(2) == 0}
		if r.IntN(2) == 0 {
			facts.FailSafeCertificate = "certified"
		}
		components[c] = facts
	}
	return Input{Loop: pick(r, propertyLoops), Round: len(rounds) - 1, Rounds: rounds, Components: components, Config: cfg}
}

func openCritical(j Judgment) bool {
	return slices.ContainsFunc(j.Findings, func(f Finding) bool { return f.Severity == SeverityCritical && f.Status == StatusOpen })
}

func landsARound(a Action) bool {
	return a == ActionLand || a == ActionSplit || a == ActionAcceptWithLimits
}

func TestProperty_NoLandingHoldsAnOpenCriticalAndTheGuardNeverFires(t *testing.T) {
	for _, residue := range []bool{false, true} {
		t.Run(fmt.Sprintf("residue=%v", residue), func(t *testing.T) { sweepLandings(t, residue) })
	}
}

func sweepLandings(t *testing.T, residue bool) {
	r := rand.New(rand.NewPCG(propertySeed, propertySeed))
	valid := 0
	landed := map[Action]int{}
	for n := 0; n < propertyInputs; n++ {
		in := randomInput(r, residue)
		if in.Validate() != nil {
			continue
		}
		valid++
		d := Decide(in)
		if landsARound(d.Action) {
			landed[d.Action]++
			if openCritical(in.Rounds[d.LandRound]) {
				t.Fatalf("input %d (%s): %s lands round %d, which holds an open CRITICAL: %+v", n, in.Loop, d.Action, d.LandRound, in.Rounds)
			}
		}
		if slices.ContainsFunc(d.Reasons, isLandingGuardReason) {
			t.Fatalf("input %d (%s): the landing guard fired, so keep-best chose a round with an open CRITICAL: %+v (decision %+v)", n, in.Loop, in.Rounds, d)
		}
	}
	t.Logf("inputs=%d valid=%d landed=%v", propertyInputs, valid, landed)
	if landed[ActionLand] == 0 {
		t.Errorf("the sweep never reached %s: the generator no longer covers the exits", ActionLand)
	}
	if !residue {
		for _, a := range []Action{ActionSplit, ActionAcceptWithLimits} {
			if landed[a] == 0 {
				t.Errorf("the sweep never reached %s: the generator no longer covers the exits", a)
			}
		}
	}
}
