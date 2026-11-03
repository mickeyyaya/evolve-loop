package cliroute

import (
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func (v resolver) requireReportDeliverable(req Request, rule string, plan llmroute.Plan) (llmroute.Plan, []string, error) {
	if req.NeedsTools || v.host.CloudHosts == nil {
		return plan, nil, nil
	}
	var hosts []string
	kept, dropped := llmroute.DropDrivers(plan, func(driver string) bool {
		h, cloud := v.host.CloudHosts(driver)
		if cloud {
			hosts = append(hosts, h...)
		}
		return cloud
	})
	switch {
	case len(dropped) == 0:
		return plan, nil, nil
	case len(kept.Candidates) == 0:
		return plan, nil, &DeliverableError{Rule: rule, Dropped: dropped, Hosts: slices.Compact(slices.Sorted(slices.Values(hosts)))}
	}
	return kept, []string{fmt.Sprintf("%v dropped: a cloud seat delivers a pull request, and this phase delivers a report", dropped)}, nil
}

func dropPullRequestOnly(v resolver, res resolution) (resolution, error) {
	plan, notes, err := v.requireReportDeliverable(res.req, res.rule, res.plan)
	if err != nil {
		return res, err
	}
	res.plan = plan
	for _, note := range notes {
		res = res.noted("%s", note)
	}
	return res, nil
}
