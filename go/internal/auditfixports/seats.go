package auditfixports

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
)

const (
	FixerProfile = "console-builder"
	freshPrefix  = "fresh:"
	auditReport  = "audit-report.md"
)

type SeatResult struct {
	RunLabel string
	Family   string
	Verdict  string
	Critical int
	High     int
	Error    string
}

type AgentRun func(ctx context.Context, args []string, input string) (SeatResult, error)

type Route func(profile string) (auditfix.Seat, error)

func freshSeat(route Route, profile string) (auditfix.Seat, error) {
	seat, err := route(profile)
	if err != nil {
		return auditfix.Seat{}, fmt.Errorf("auditfixports: route %s: %w", profile, err)
	}
	seat.Profile, seat.Session = profile, freshPrefix+profile
	return seat, nil
}
