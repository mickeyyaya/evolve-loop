package swarm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParsePlan extracts the SwarmPlan from the first ```json block of a swarm-plan.md artifact, or from bare JSON.
func ParsePlan(artifact string) (SwarmPlan, error) {
	block, err := firstJSONBlock(artifact)
	if err != nil {
		return SwarmPlan{}, err
	}
	var env planEnvelope
	if err := json.Unmarshal([]byte(block), &env); err != nil {
		return SwarmPlan{}, fmt.Errorf("swarm-plan JSON: %w", err)
	}
	plan := env.SwarmPlan
	if plan.Mode == "" {
		return SwarmPlan{}, fmt.Errorf("swarm-plan missing required \"mode\" (writer|reader)")
	}
	return plan, nil
}

func firstJSONBlock(s string) (string, error) {
	const fence = "```"
	lower := strings.ToLower(s)
	if i := strings.Index(lower, fence+"json"); i >= 0 {
		rest := s[i+len(fence)+len("json"):]
		if j := strings.Index(rest, fence); j >= 0 {
			return strings.TrimSpace(rest[:j]), nil
		}
		return "", fmt.Errorf("swarm-plan: unterminated ```json block")
	}
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}
	return "", fmt.Errorf("swarm-plan: no ```json block found")
}
