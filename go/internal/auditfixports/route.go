package auditfixports

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func RouterRoute(resolve func(cliroute.Request) (cliroute.Decision, error), projectRoot string) Route {
	return func(profile string) (auditfix.Seat, error) {
		d, err := resolve(cliroute.Request{Agent: profile, Phase: profile, ProjectRoot: projectRoot, NeedsTools: true})
		if err != nil {
			return auditfix.Seat{}, err
		}
		if len(d.Plan.Candidates) == 0 {
			return auditfix.Seat{}, fmt.Errorf("the routing of %s has no candidate CLI", profile)
		}
		return auditfix.Seat{Family: llmroute.Family(d.Plan.Candidates[0]), Tier: d.Plan.Model}, nil
	}
}
