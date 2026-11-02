package auditfixports

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
)

const (
	FixerProfile = "console-builder"
	freshPrefix  = "fresh:"
	auditReport  = "audit-report.md"
)

type AgentRun func(ctx context.Context, args []string, input string) (agentcmd.Envelope, error)

type Route func(profile string) (auditfix.Seat, error)

func freshSeat(route Route, profile string) (auditfix.Seat, error) {
	seat, err := route(profile)
	if err != nil {
		return auditfix.Seat{}, fmt.Errorf("auditfixports: route %s: %w", profile, err)
	}
	seat.Profile, seat.Session = profile, freshPrefix+profile
	return seat, nil
}
