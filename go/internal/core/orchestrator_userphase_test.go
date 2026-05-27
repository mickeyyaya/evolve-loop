package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// userPhaseOrchestrator builds an orchestrator whose catalog carries three user
// phases and whose routing order splices them between build and audit.
func userPhaseOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	cat, _ := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "security-scan", Optional: true, After: "build"},
		{Name: "writer", Optional: true, WritesSource: true},
		{Name: "mand", Optional: false}, // floor-violating: must never be a legal target
	})
	cfg := config.RoutingConfig{
		Stage: config.StageEnforce,
		Order: []string{"scout", "build", "security-scan", "writer", "audit", "ship"},
	}
	return NewOrchestrator(nil, nil, nil, WithCatalog(cat), WithRouting(cfg, nil))
}

func TestCandidatePhase(t *testing.T) {
	o := userPhaseOrchestrator(t)
	cases := []struct {
		in   string
		want Phase
	}{
		{"scout", PhaseScout},              // built-in
		{"security-scan", "security-scan"}, // user phase in catalog
		{"retrospective", PhaseRetro},      // canonical → alias
		{"nonexistent", ""},                // unknown → declined
	}
	for _, c := range cases {
		if got := o.candidatePhase(c.in); got != c.want {
			t.Errorf("candidatePhase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTransitionLegal_UserPhases(t *testing.T) {
	o := userPhaseOrchestrator(t)
	cases := []struct {
		from, cand Phase
		want       bool
	}{
		{PhaseBuild, "security-scan", true},  // build → user (forward, optional)
		{"security-scan", PhaseAudit, true},  // user → audit (forward to anchor)
		{"security-scan", "writer", true},    // user → user (forward)
		{PhaseBuild, "mand", false},          // non-optional user phase rejected (floor)
		{PhaseAudit, "security-scan", false}, // backward in order → illegal
		{PhaseBuild, "unknown-phase", false}, // absent from order/catalog
	}
	for _, c := range cases {
		if got := o.transitionLegal(c.from, c.cand); got != c.want {
			t.Errorf("transitionLegal(%q, %q) = %v, want %v", c.from, c.cand, got, c.want)
		}
	}
}

func TestWorktreePhase_FromSpec(t *testing.T) {
	o := userPhaseOrchestrator(t)
	cases := []struct {
		p    Phase
		want bool
	}{
		{PhaseBuild, true},       // built-in source writer
		{PhaseTDD, true},         // built-in source writer
		{PhaseAudit, false},      // built-in non-writer
		{"writer", true},         // user phase with writes_source
		{"security-scan", false}, // user phase, no writes_source
	}
	for _, c := range cases {
		if got := o.worktreePhase(c.p); got != c.want {
			t.Errorf("worktreePhase(%q) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestNextInOrder(t *testing.T) {
	o := userPhaseOrchestrator(t)
	if got := o.nextInOrder("security-scan"); got != "writer" {
		t.Errorf("nextInOrder(security-scan) = %q, want writer", got)
	}
	if got := o.nextInOrder("ship"); got != PhaseEnd {
		t.Errorf("nextInOrder(ship) = %q, want end", got)
	}
	if got := o.nextInOrder("absent"); got != PhaseEnd {
		t.Errorf("nextInOrder(absent) = %q, want end", got)
	}
}
