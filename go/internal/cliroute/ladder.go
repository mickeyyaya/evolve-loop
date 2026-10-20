package cliroute

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func applyLadder(v resolver, res resolution) (resolution, error) {
	ov := res.req.Overlay
	if ov.LadderTier == "" && ov.Effort == "" {
		return res, nil
	}
	rung, notes := clampLadder(llmroute.Overlay{LadderTier: ov.LadderTier, Effort: ov.Effort}, v.table.bounds[res.req.Agent].Resolved())
	for _, note := range notes {
		res = res.noted("%s", note)
	}
	res.plan = llmroute.ApplySoftOverlay(res.plan, rung, res.prof)
	return res, nil
}

func clampLadder(ov llmroute.Overlay, bounds retryladder.Bounds) (llmroute.Overlay, []string) {
	var notes []string
	if ov.LadderTier != "" && policy.TierRank(ov.LadderTier) > policy.TierRank(bounds.MaxTier) {
		notes = append(notes, fmt.Sprintf("ladder tier %s clamped to max_tier %s", ov.LadderTier, bounds.MaxTier))
		ov.LadderTier = bounds.MaxTier
	}
	levels := policy.EffortLevels()
	if ov.Effort != "" && slices.Index(levels, ov.Effort) > slices.Index(levels, bounds.MaxEffort) {
		notes = append(notes, fmt.Sprintf("ladder effort %s clamped to max_effort %s", ov.Effort, bounds.MaxEffort))
		ov.Effort = bounds.MaxEffort
	}
	return ov, notes
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
