package convergence

import (
	"fmt"
	"strconv"
	"strings"
)

func signal(code string, fields ...string) string {
	var b strings.Builder
	b.WriteString(code)
	for i := 0; i+1 < len(fields); i += 2 {
		b.WriteString(" " + fields[i] + "=" + fields[i+1])
	}
	return b.String()
}

func (c decider) reason(code string, fields ...string) string {
	return signal(code, append([]string{"loop", string(c.in.Loop)}, fields...)...)
}

func (c decider) rungReason(d Decision) string {
	cause := c.cause
	if d.Action == ActionLand {
		_, cause = c.landRung()
	}
	return c.reason("CONVERGENCE_RUNG", "round", strconv.Itoa(c.in.Round), "rung", strconv.Itoa(d.Rung),
		"bar", string(d.BlockingBar), "action", string(d.Action), "cause", cause)
}

func (c decider) roundGainReasons() []string {
	r := c.in.Round
	if r == 0 || c.in.Rounds[r].Reentry {
		return nil
	}
	g := c.gainOf(r)
	var reasons []string
	if g.noProgress() {
		reasons = append(reasons, c.reason("CONVERGENCE_NO_PROGRESS", "round", strconv.Itoa(r), "mass_prev", formatMass(g.massPrev),
			"mass", formatMass(g.mass), "bar", string(g.bar)))
	}
	if g.damaging() {
		reasons = append(reasons, c.reason("CONVERGENCE_REPAIR_DAMAGE", "round", strconv.Itoa(r), "damage", strconv.Itoa(g.damage),
			"repairs", strconv.Itoa(g.repairs)))
	}
	return reasons
}

func (c decider) concentrationReasons() []string {
	if !c.hot.fired {
		return nil
	}
	return []string{c.reason("CONVERGENCE_CONCENTRATION", "component", c.hot.component, "share", formatShare(c.hot.share),
		"prev_share", formatShare(c.hot.prev), "window", strconv.Itoa(c.in.Config.ConcentrationWindow))}
}

func (c decider) noHeadroomReason(role string, from TierEffort, to string) string {
	if to == "" {
		to = "none"
	}
	return c.reason("CONVERGENCE_NO_HEADROOM", "role", role, "family", from.Family, "from", from.Tier, "to", to)
}

func (c decider) settleReasons(filed []Finding, exit scopeExit, deferred int) []string {
	var reasons []string
	if len(filed) > 0 {
		reasons = append(reasons, c.filedReason(filed))
	}
	if exit.split != "" {
		reasons = append(reasons, c.splitReason(exit.split))
	}
	for _, component := range exit.accepted {
		reasons = append(reasons, c.reason("CONVERGENCE_ACCEPTED_LIMITS", "component", component,
			"certificate", c.in.Components[component].FailSafeCertificate))
	}
	if deferred > 0 {
		reasons = append(reasons, c.reason("CONVERGENCE_DEFERRED", "count", strconv.Itoa(deferred), "followups", strconv.Itoa(deferred)))
	}
	return reasons
}

func (c decider) stopReasons(split string, open, landRound int) []string {
	var reasons []string
	if split != "" {
		reasons = append(reasons, c.splitReason(split))
	}
	return append(reasons, c.reason("CONVERGENCE_STOP", "round", strconv.Itoa(c.in.Round), "open", strconv.Itoa(open),
		"land_round", strconv.Itoa(landRound)))
}

func (c decider) splitReason(component string) string {
	return c.reason("CONVERGENCE_SPLIT", "component", component, "followup", component)
}

func (c decider) filedReason(filed []Finding) string {
	var late, capability, info int
	for _, f := range filed {
		switch {
		case f.Severity == SeverityInfo:
			info++
		case f.Kind == KindCapability:
			capability++
		default:
			late++
		}
	}
	return c.reason("CONVERGENCE_FILED", "late", strconv.Itoa(late), "capability", strconv.Itoa(capability), "info", strconv.Itoa(info))
}

func formatMass(m float64) string { return strconv.FormatFloat(m, 'f', -1, 64) }

func formatShare(s float64) string { return fmt.Sprintf("%.3f", s) }
