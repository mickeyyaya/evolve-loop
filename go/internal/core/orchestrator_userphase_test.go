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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// Issue #9: the AUDIT phase must run with cwd=worktree so its read-only
// verification commands (git diff HEAD, go test, test -d) inspect the builder's
// pending work in the worktree — a non-Claude auditor running a relative `cd go`
// from the project root saw an empty main tree and false-FAILed work that was
// present in the worktree. Post-CB.1 the worktree CWD is universal (pinned by
// TestCB1_EveryDispatchedPhaseCarriesWorktree); what this table pins is the
// WRITE axis — worktreePhase / role-gate permission stays exactly the source
// writers, audit and the read-only spine included as non-writers.
func TestWorktreePhase_WriteAxisIsSourceWritersOnly(t *testing.T) {
	t.Parallel()
	o := userPhaseOrchestrator(t)
	writeCases := []struct {
		p    Phase
		want bool
	}{
		{PhaseBuild, true},  // source writer
		{PhaseTDD, true},    // source writer
		{PhaseAudit, false}, // read-only inspector (cwd=worktree, never a writer)
		{PhaseScout, false}, // read-only discovery
		{PhaseRetro, false}, // reads workspace artifacts
	}
	for _, c := range writeCases {
		if got := o.worktreePhase(c.p); got != c.want {
			t.Errorf("worktreePhase(%q) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestNextInOrder(t *testing.T) {
	t.Parallel()
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
