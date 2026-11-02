package loopwave

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

type WavePlan struct {
	Width    int             `json:"width"`
	Reason   string          `json:"reason"`
	Lanes    []PlanLane      `json:"lanes"`
	Excluded []PlanExclusion `json:"excluded"`
}

type PlanLane struct {
	Items []PlanItem `json:"items"`
}

type PlanItem struct {
	ID     string  `json:"id"`
	Weight float64 `json:"weight"`
}

type PlanExclusion struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

func (e *Engine) Preview(ctx context.Context, fc policy.FleetConfig, routing *policy.CLIRouting, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) WavePlan {
	scan, _ := inboxbatch.ScanDir(e.lifecycle().InboxDir)
	plan := WavePlan{Width: fc.Count, Lanes: []PlanLane{}, Excluded: e.excluded(scan.Items)}
	if !ShouldRunWave(fc) {
		plan.Reason = notWaveReason(fc)
		return plan
	}
	_, paused := e.QuotaCount(fc, routing)
	sized, _ := e.Size(fc, routing, states, tp, now)
	count := sized.Count
	plan.Width = count
	plan.Reason = widthReason(fc, count, paused)
	req := DispatchRequest{Config: sized, Wave: 1, Plan: e.planFn(count, false), Routed: e.RoutedResolver()}
	_, specs, err := planLanes(ctx, req, count)
	if err != nil {
		plan.Reason = err.Error()
		return plan
	}
	weights := make(map[string]float64, len(scan.Items))
	for _, it := range scan.Items {
		weights[it.ID] = it.Weight
	}
	for _, spec := range specs {
		lane := PlanLane{Items: make([]PlanItem, len(spec.Scope))}
		for i, id := range spec.Scope {
			lane.Items[i] = PlanItem{ID: id, Weight: weights[id]}
		}
		plan.Lanes = append(plan.Lanes, lane)
	}
	return plan
}

func (e *Engine) excluded(items []inboxbatch.Item) []PlanExclusion {
	lifecycle := e.lifecycle()
	routed := e.routedBase()
	out := []PlanExclusion{}
	for _, it := range items {
		if isRouted, reason := routed(it.ID); isRouted {
			out = append(out, PlanExclusion{ID: it.ID, Reason: "console-routed: " + reason})
		} else if d := inboxmover.ResolveDispatchability(lifecycle, it.ID); !d.Dispatchable {
			out = append(out, PlanExclusion{ID: it.ID, Reason: d.Reason})
		}
	}
	return out
}

func notWaveReason(fc policy.FleetConfig) string {
	switch {
	case fc.Count <= 1:
		return fmt.Sprintf("sequential: fleet.count is %d, a wave needs more than 1 lane", fc.Count)
	case fc.PlanSource != "triage":
		return fmt.Sprintf("sequential: fleet.plan_source is %q, a wave needs \"triage\"", fc.PlanSource)
	default:
		return "pool scheduling: lanes are drawn per slot from the supply, no wave plan is made"
	}
}

func widthReason(fc policy.FleetConfig, realized int, paused []string) string {
	configured := fc.Count
	switch {
	case realized == configured:
		return fmt.Sprintf("wave of %d lane(s): fleet.count=%d", realized, configured)
	case len(paused) > 0:
		return fmt.Sprintf("wave of %d lane(s): fleet.count=%d shrank for benched families %s", realized, configured, strings.Join(paused, ","))
	default:
		return fmt.Sprintf("wave of %d lane(s): fleet.count=%d sized by the fleet budget (stage %s)", realized, configured, fc.Budget.Stage)
	}
}
