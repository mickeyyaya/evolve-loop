package cliroute

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	OrderHeadroom      = "headroom"
	DefaultHeadroomPct = 60
	maxHeadroomPct     = 100
)

var (
	errNoUsageReader = errors.New("the host has no usage reader")
	errNoWeekWindow  = errors.New("no week window is recorded for a family of the chain")
)

func (c *compiler) compileOrders() map[string]int {
	orders := map[string]int{}
	for _, agent := range sortedKeys(c.agentKeys) {
		key := c.agentKeys[agent]
		if pct, ok := c.compileOrder("cli_routing.agents."+key, c.block.Agents[key]); ok {
			orders[agent] = pct
		}
	}
	return orders
}

func (c *compiler) compileOrder(path string, rule policy.AgentRule) (int, bool) {
	switch {
	case rule.Order == "" && rule.HeadroomPct == 0:
		return 0, false
	case rule.Order == "":
		c.add(SeverityError, path+".headroom_pct", "headroom_pct needs order %s on the same rule", OrderHeadroom)
	case rule.Order != OrderHeadroom:
		c.add(SeverityError, path+".order", "unknown order %q (orders: %s)", rule.Order, OrderHeadroom)
	case len(rule.CLI) == 0:
		c.add(SeverityError, path+".order", "order %s sorts the chain of this rule, so the rule must list its cli", OrderHeadroom)
	case rule.HeadroomPct < 0 || rule.HeadroomPct > maxHeadroomPct:
		c.add(SeverityError, path+".headroom_pct", "%d is outside 1 to %d (0 or absent means %d)", rule.HeadroomPct, maxHeadroomPct, DefaultHeadroomPct)
	case rule.HeadroomPct == 0:
		return DefaultHeadroomPct, true
	default:
		return rule.HeadroomPct, true
	}
	return 0, false
}

func orderByHeadroom(v resolver, res resolution) (resolution, error) {
	pct, ordered := v.table.orders[res.req.Agent]
	if !ordered {
		return res, nil
	}
	used, err := v.chainWeekUsed(res.req.ProjectRoot, res.plan.Candidates)
	if err != nil {
		v.logf("[cliroute] agent=%s order=%s: %v, so the policy order stays\n", res.req.Agent, OrderHeadroom, err)
		return res.noted("headroom: %v, so the policy order stays", err), nil
	}
	lanes, err := v.laneTimes(res.req.ProjectRoot)
	if err != nil {
		v.logf("[cliroute] agent=%s order=%s: lane time: %v, so no family has a measure\n", res.req.Agent, OrderHeadroom, err)
		res = res.noted("lane time: %v, so no family has a measure", err)
	}
	perPass := perPassOf(lanes)
	res.plan.Candidates = headroomOrder(res.plan.Candidates, used, perPass, pct)
	fams := familiesOf(res.plan.Candidates)
	return res.noted("headroom %d%%: %s", pct, weekNote(fams, used, pct)).noted("lane time to a PASS: %s", laneNote(fams, lanes)), nil
}

func (v resolver) chainWeekUsed(root string, chain []string) (map[string]float64, error) {
	if v.host.WeekUsed == nil {
		return nil, errNoUsageReader
	}
	used, err := v.host.WeekUsed(root)
	if err != nil {
		return nil, fmt.Errorf("the usage read failed: %w", err)
	}
	for _, fam := range familiesOf(chain) {
		if _, known := used[fam]; known {
			return used, nil
		}
	}
	return nil, errNoWeekWindow
}

func (v resolver) laneTimes(root string) ([]lanetime.FamilyLane, error) {
	if v.host.LaneTimes == nil {
		return nil, nil
	}
	return v.host.LaneTimes(root)
}

func perPassOf(lanes []lanetime.FamilyLane) map[string]time.Duration {
	out := map[string]time.Duration{}
	for _, l := range lanes {
		out[l.Family] = l.PerPass
	}
	return out
}

func headroomOrder(chain []string, used map[string]float64, perPass map[string]time.Duration, pct int) []string {
	out := slices.Clone(chain)
	slices.SortStableFunc(out, func(a, b string) int {
		fa, fb := familyOf(a), familyOf(b)
		return cmp.Or(
			cmp.Compare(rankOf(atLimit(used, fa, pct)), rankOf(atLimit(used, fb, pct))),
			compareMeasure(perPass[fa], perPass[fb]),
			cmp.Compare(unmeasuredRank(fa), unmeasuredRank(fb)),
		)
	})
	return out
}

func atLimit(used map[string]float64, family string, pct int) bool {
	u, known := used[family]
	return known && u >= float64(pct)
}

func rankOf(late bool) int {
	if late {
		return 1
	}
	return 0
}

func compareMeasure(a, b time.Duration) int {
	if a == 0 || b == 0 {
		return cmp.Compare(rankOf(a == 0), rankOf(b == 0))
	}
	return cmp.Compare(a, b)
}

func unmeasuredRank(family string) int {
	order := policy.UnmeasuredOrder()
	if i := slices.Index(order, family); i >= 0 {
		return i
	}
	return len(order)
}

func weekNote(fams []string, used map[string]float64, pct int) string {
	parts := make([]string, 0, len(fams))
	for _, fam := range fams {
		u, known := used[fam]
		switch {
		case !known:
			parts = append(parts, fam+" week unknown")
		case atLimit(used, fam, pct):
			parts = append(parts, fmt.Sprintf("%s week %g%% (at or above the limit: leaves the head)", fam, u))
		default:
			parts = append(parts, fmt.Sprintf("%s week %g%%", fam, u))
		}
	}
	return strings.Join(parts, " · ")
}

func laneNote(fams []string, lanes []lanetime.FamilyLane) string {
	byFamily := map[string]lanetime.FamilyLane{}
	for _, l := range lanes {
		if l.PerPass > 0 {
			byFamily[l.Family] = l
		}
	}
	if len(byFamily) == 0 {
		return fmt.Sprintf("no family has a measure, so the order is %s", strings.Join(policy.UnmeasuredOrder(), ", "))
	}
	parts := make([]string, 0, len(fams))
	for _, fam := range fams {
		l, measured := byFamily[fam]
		if !measured {
			parts = append(parts, fam+" no measure")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s per PASS (%d builds, %d passes)", fam, l.PerPass.Round(time.Second), l.Builds, l.Passes))
	}
	return strings.Join(parts, " · ")
}
