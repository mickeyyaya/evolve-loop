package inboxbatch

import (
	"fmt"
	"strings"
)

// UnifiedMember names one inbox item and its evidence for the shared root cause.
type UnifiedMember struct {
	ID       string `json:"id"`
	Evidence string `json:"evidence"`
}

// UnifiedCommitment is triage's claim that several items close through one shared design; it never alters grouping.
type UnifiedCommitment struct {
	RootCauseHypothesis string          `json:"root_cause_hypothesis"`
	SharedSeam          string          `json:"shared_seam"`
	DesignRequirements  []string        `json:"design_requirements"`
	Members             []UnifiedMember `json:"members"`
}

// Validate rejects incomplete, unknown, duplicated or mixed-campaign/kind members; callers fall back to independent selection.
func (c UnifiedCommitment) Validate(items []Item) error {
	if strings.TrimSpace(c.RootCauseHypothesis) == "" {
		return fmt.Errorf("unified commitment: root cause hypothesis is empty")
	}
	if strings.TrimSpace(c.SharedSeam) == "" {
		return fmt.Errorf("unified commitment: shared seam is empty")
	}
	if len(c.DesignRequirements) == 0 {
		return fmt.Errorf("unified commitment: design requirements are empty")
	}
	for i, requirement := range c.DesignRequirements {
		if strings.TrimSpace(requirement) == "" {
			return fmt.Errorf("unified commitment: design requirement %d is empty", i+1)
		}
	}
	if len(c.Members) < 2 {
		return fmt.Errorf("unified commitment: at least two members are required")
	}

	known := make(map[string]Item, len(items))
	for _, item := range items {
		known[item.ID] = item
	}
	seen := make(map[string]bool, len(c.Members))
	campaigns := map[string]bool{}
	kinds := map[string]bool{}
	for _, member := range c.Members {
		id := strings.TrimSpace(member.ID)
		if id == "" {
			return fmt.Errorf("unified commitment: member id is empty")
		}
		if strings.TrimSpace(member.Evidence) == "" {
			return fmt.Errorf("unified commitment: member %q has no evidence", id)
		}
		if seen[id] {
			return fmt.Errorf("unified commitment: member %q is duplicated", id)
		}
		seen[id] = true
		item, ok := known[id]
		if !ok {
			return fmt.Errorf("unified commitment: member %q is not a known inbox item", id)
		}
		campaigns[strings.TrimSpace(item.Campaign)] = true
		kind := strings.TrimSpace(item.DeliverableKind)
		if kind == "" {
			kind = "code"
		}
		kinds[kind] = true
	}
	if len(campaigns) > 1 {
		return fmt.Errorf("unified commitment: members span distinct campaigns")
	}
	if len(kinds) > 1 {
		return fmt.Errorf("unified commitment: members mix deliverable kinds")
	}
	return nil
}

// Size reports whether the commitment fits one cycle's existing batch cap.
func (c UnifiedCommitment) Size() string {
	if len(c.Members) <= DefaultMaxItems {
		return "small"
	}
	return "large"
}
