package advisor

import (
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const (
	tierBalanced = "balanced"
	tierDeep     = "deep"
)

func applyDeepBuildRisk(in router.RouteInput, plan *router.PhasePlan) {
	if len(in.Cfg.DeepBuildRisk) == 0 {
		return
	}
	evidence := deepBuildEvidence(in)
	if evidence == "" {
		return
	}
	tier := tierBalanced
	if matchesRiskClass(in.Cfg.DeepBuildRisk, evidence) {
		tier = tierDeep
	}
	for i := range plan.Entries {
		if plan.Entries[i].Phase == "build" && plan.Entries[i].Run {
			plan.Entries[i].Tier = tier
		}
	}
}

func deepBuildEvidence(in router.RouteInput) string {
	var b strings.Builder
	for _, item := range in.LaneItems {
		if item.Unresolved != "" {
			continue
		}
		b.WriteString(strings.Join(item.Acceptance, "\n"))
		b.WriteString("\n")
		b.WriteString(strings.Join(item.Files, "\n"))
		b.WriteString("\n")
	}
	if text := strings.TrimSpace(b.String()); text != "" {
		return strings.ToLower(text)
	}
	return strings.ToLower(strings.TrimSpace(in.GoalText))
}

func matchesRiskClass(classes map[string][]string, evidence string) bool {
	for _, markers := range classes {
		for _, marker := range markers {
			if strings.Contains(evidence, marker) {
				return true
			}
		}
	}
	return false
}

func riskClassNames(classes map[string][]string) string {
	return strings.Join(slices.Sorted(maps.Keys(classes)), ", ")
}
