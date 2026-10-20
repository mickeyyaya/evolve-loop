package cliroute

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type ladderFloor struct {
	tier, effort string
}

func applyLadder(v resolver, res resolution) (resolution, error) {
	ov := res.req.Overlay
	if ov.LadderTier == "" && ov.Effort == "" {
		return res, nil
	}
	rung, notes, err := clampLadder(llmroute.Overlay{LadderTier: ov.LadderTier, Effort: ov.Effort}, v.table.bounds[res.req.Agent].Resolved(), v.table.ladderBase(res.req.Agent, res.plan.Model))
	if err != nil {
		return res, err
	}
	for _, note := range notes {
		res = res.noted("%s", note)
	}
	res.plan = llmroute.ApplySoftOverlay(res.plan, rung, res.prof)
	return res, nil
}

func (t Table) ladderBase(agent, tier string) ladderFloor {
	effort, _ := t.pol.Efforts().Resolve(tier, agent, t.agentKeys[agent])
	return ladderFloor{tier: tier, effort: effort}
}

func clampLadder(ov llmroute.Overlay, bounds retryladder.Bounds, base ladderFloor) (llmroute.Overlay, []string, error) {
	tier, tierNote, err := clampStep("tier", ov.LadderTier, policy.TierNames(), bounds.MaxTier, base.tier)
	if err != nil {
		return ov, nil, err
	}
	effort, effortNote, err := clampStep("effort", ov.Effort, policy.EffortLevels(), bounds.MaxEffort, base.effort)
	if err != nil {
		return ov, nil, err
	}
	ov.LadderTier, ov.Effort = tier, effort
	return ov, slices.DeleteFunc([]string{tierNote, effortNote}, func(n string) bool { return n == "" }), nil
}

func clampStep(kind, rung string, levels []string, bound, base string) (string, string, error) {
	if rung == "" {
		return "", "", nil
	}
	at, floor, ceiling := slices.Index(levels, rung), slices.Index(levels, base), slices.Index(levels, bound)
	switch {
	case at < 0:
		return "", "", fmt.Errorf("ladder %s %q is not one of %s", kind, rung, strings.Join(levels, ", "))
	case floor >= 0 && at < floor:
		return base, fmt.Sprintf("ladder %s %s raised to the base %s %s", kind, rung, kind, base), nil
	case max(ceiling, floor) >= 0 && at > max(ceiling, floor):
		limit, label := bound, "max_"+kind
		if ceiling < floor {
			limit, label = base, "the base "+kind
		}
		return limit, fmt.Sprintf("ladder %s %s clamped to %s %s", kind, rung, label, limit), nil
	}
	return rung, "", nil
}

func (c *compiler) compileLadderBounds() map[string]retryladder.Bounds {
	bounds := map[string]retryladder.Bounds{}
	for _, agent := range sortedKeys(c.agentKeys) {
		key := c.agentKeys[agent]
		rule, path := c.block.Agents[key], "cli_routing.agents."+key
		if rule.MaxTier != "" && !slices.Contains(policy.TierNames(), rule.MaxTier) {
			c.add(SeverityError, path+".max_tier", "unknown tier %q (tiers: %s)", rule.MaxTier, strings.Join(policy.TierNames(), ", "))
		}
		if rule.MaxEffort != "" {
			if err := policy.ValidateEffort(rule.MaxEffort); err != nil {
				c.add(SeverityError, path+".max_effort", "%v", err)
			}
		}
		bounds[agent] = retryladder.Bounds{MaxTier: rule.MaxTier, MaxEffort: rule.MaxEffort}
	}
	return bounds
}

func inheritsChain(rule policy.AgentRule) bool {
	return len(rule.CLI) == 0 && (rule.Effort != "" || rule.MaxTier != "" || rule.MaxEffort != "")
}
