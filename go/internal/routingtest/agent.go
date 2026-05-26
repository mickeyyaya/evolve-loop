package routingtest

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// scriptedProposer is the deterministic stand-in for the LLM router brain. Keyed
// by the just-completed phase (RouteInput.Current), it returns a canned proposal
// or a forced error (to exercise LLMProposal's degrade-to-static path). A nil
// proposal for a phase is legal — the kernel decision stands unchanged.
type scriptedProposer struct {
	spec  AgentSpec
	calls int
	seen  []string
}

func (p *scriptedProposer) Propose(in router.RouteInput) (*router.Proposal, error) {
	p.calls++
	p.seen = append(p.seen, in.Current)
	if p.spec.ErrorsOn[in.Current] {
		return nil, fmt.Errorf("scripted proposer degrade at %s", in.Current)
	}
	return p.spec.Proposals[in.Current], nil
}

var _ router.Proposer = (*scriptedProposer)(nil)
