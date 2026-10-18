package cliroute

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

const toolUseUnjudgedNote = "the host does not judge tool use, so the chain of a worktree phase is not filtered"

func (v resolver) requireToolUse(req Request, rule string, plan llmroute.Plan) (llmroute.Plan, []string, error) {
	if !req.NeedsTools {
		return plan, nil, nil
	}
	if v.host.Toolless == nil {
		return plan, []string{toolUseUnjudgedNote}, nil
	}
	kept, dropped := llmroute.DropToolless(plan, v.host.Toolless)
	switch {
	case len(dropped) == 0:
		return plan, nil, nil
	case len(kept.Candidates) == 0:
		return plan, nil, &ToolUseError{Rule: rule, Dropped: dropped}
	}
	return kept, []string{fmt.Sprintf("%v dropped: no tool use for a worktree phase", dropped)}, nil
}

func dropToolless(v resolver, res resolution) (resolution, error) {
	plan, notes, err := v.requireToolUse(res.req, res.rule, res.plan)
	if err != nil {
		return res, err
	}
	res.plan = plan
	for _, note := range notes {
		res = res.noted("%s", note)
	}
	return res, nil
}
