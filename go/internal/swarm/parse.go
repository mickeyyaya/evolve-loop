package swarm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParsePlan extracts the SwarmPlan from a swarm-plan.md artifact. The planner
// persona emits a fenced ```json block wrapping {"swarm_plan": {...}}; we take
// the first such block. Falling back, if the whole artifact is bare JSON we
// parse that directly (keeps the parser robust to a CLI that omits the fence).
//
// Pure and I/O-free: the caller reads the file. Returns an error the caller
// treats as "no usable plan" → N=1 fallback, never a hard cycle abort.
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

// firstJSONBlock returns the contents of the first ```json ... ``` fence, or the
// whole trimmed input if no fence is present (and it looks like JSON).
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
