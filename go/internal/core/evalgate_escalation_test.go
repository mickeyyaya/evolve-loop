package core

import (
	"context"
	"strings"
	"testing"
)

// evalGateProbe models the evalgate reviewer's rejection shape: Blocks==0
// (no contract-gate breaker), Remediation carried, identical reason per round.
type evalGateProbe struct {
	phase        string
	compliantCLI string // "" = never complies (the live 0-for-N shape)
	reasons      []string
	remediation  string

	lastCLI    string
	dispatched []string
	rejections int
}

func (p *evalGateProbe) Name() string { return p.phase }

func (p *evalGateProbe) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.lastCLI = req.ModelRoutingCLI
	p.dispatched = append(p.dispatched, req.ModelRoutingCLI)
	return PhaseResponse{Phase: p.phase, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func (p *evalGateProbe) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != p.phase {
		return ReviewResult{Approve: true}
	}
	if p.compliantCLI != "" && p.lastCLI == p.compliantCLI {
		return ReviewResult{Approve: true}
	}
	reason := "scout did not materialize evals for selected slug(s): a-slug, b-slug"
	if n := len(p.reasons); n > 0 {
		idx := p.rejections
		if idx >= n {
			idx = n - 1
		}
		reason = p.reasons[idx]
	}
	p.rejections++
	return ReviewResult{Approve: false, Reason: reason, Blocks: 0, Remediation: p.remediation}
}

func runEvalGateCycle(t *testing.T, probe *evalGateProbe) *evalGateProbe {
	t.Helper()
	root := t.TempDir()
	// A non-claude profile gives escalation a real target; with no profile the
	// phase resolves to the universal claude family, and the salvage-retry arm
	// fires instead (a different behavior, covered by its own tests).
	writeCLIProfile(t, root, "builder", "codex-tmux", []string{"claude-tmux"})
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(probe))
	_, _ = o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	return probe
}

func TestEvalGateEscalation_SecondIdenticalRejectionEscalatesAndRecovers(t *testing.T) {
	p := runEvalGateCycle(t, &evalGateProbe{
		phase:        string(PhaseBuild),
		compliantCLI: universalContractFallbackCLI,
		remediation:  "Create the missing eval file(s) at <workspace>/.evolve/evals/<slug>.md",
	})
	if len(p.dispatched) < 3 {
		t.Fatalf("want initial + correction1 (same CLI) + correction2 (escalated); got dispatches %v", p.dispatched)
	}
	if p.dispatched[1] != p.dispatched[0] {
		t.Fatalf("correction 1 must retry the SAME CLI (one bad turn is not a CLI verdict); got %v", p.dispatched)
	}
	if p.dispatched[2] != universalContractFallbackCLI {
		t.Fatalf("correction 2 must escalate to the fallback family; got %v", p.dispatched)
	}
}

func TestEvalGateEscalation_DifferentReasonsDoNotEscalate(t *testing.T) {
	p := runEvalGateCycle(t, &evalGateProbe{
		phase: string(PhaseBuild),
		reasons: []string{
			"scout did not materialize evals for selected slug(s): a-slug",
			"scout did not materialize evals for selected slug(s): z-other",
		},
		remediation: "Create the missing eval file(s)",
	})
	for i, cli := range p.dispatched {
		if cli == universalContractFallbackCLI && i > 0 {
			t.Fatalf("different defects must not escalate; dispatch %d went to %q (%v)", i, cli, p.dispatched)
		}
	}
}

func TestEvalGateEscalation_RemediationlessRejectionNeverEscalates(t *testing.T) {
	p := runEvalGateCycle(t, &evalGateProbe{
		phase:       string(PhaseBuild),
		remediation: "",
	})
	for i, cli := range p.dispatched {
		if i > 0 && cli != p.dispatched[0] {
			t.Fatalf("a remediation-less Blocks==0 rejection must never change the re-dispatch CLI; got %v", p.dispatched)
		}
	}
}

func TestEvalGateEscalation_EscalatedDirectiveKeepsTheRemediation(t *testing.T) {
	seen := []string{}
	p := &evalGateProbe{
		phase:        string(PhaseBuild),
		compliantCLI: universalContractFallbackCLI,
		remediation:  "Create the missing eval file(s) at exactly these paths",
	}
	probeRun := p.Run
	_ = probeRun
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "codex-tmux", []string{"claude-tmux"})
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = correctionCapture{p, &seen}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(p))
	_, _ = o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	if len(seen) < 3 {
		t.Fatalf("want 3 dispatches; got %d", len(seen))
	}
	if !strings.Contains(seen[2], "exactly these paths") {
		t.Fatalf("the ESCALATED dispatch must still carry the remediation; directive=%q", seen[2])
	}
}

type correctionCapture struct {
	*evalGateProbe
	directives *[]string
}

func (c correctionCapture) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	*c.directives = append(*c.directives, req.CorrectionDirective)
	return c.evalGateProbe.Run(ctx, req)
}
