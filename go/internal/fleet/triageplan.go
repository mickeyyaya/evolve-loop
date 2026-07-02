package fleet

import (
	"encoding/json"
	"fmt"
)

// triageDecision is the subset of a wave's triage-decision.json (companion
// artifact written by the triage phase; schema owned by
// internal/triagecap.ReadDeclaredFloors) that PlanFromTriage consumes.
// TopN mirrors the orchestrator's projected-decision shape (real
// triage-decision.json artifacts, e.g. .evolve/runs/cycle-464/, commonly carry
// only top_n[].id and no committed_floors at all — see PlanFromTriage doc).
type triageDecision struct {
	CommittedFloors []string `json:"committed_floors"`
	TopN            []struct {
		ID string `json:"id"`
	} `json:"top_n"`
}

// PlanFromTriage adapts one wave's single-writer triage output into disjoint
// launch specs (FLEET-AS-POLICY S2). decisionJSON is a wave's
// triage-decision.json bytes; its committed_floors become the wave's todo IDs.
// When committed_floors is absent or empty, cardPackages (the committed
// cards' target packages) is the fallback source of todo IDs. When BOTH
// committed_floors and cardPackages are empty, the decision's top_n[].id
// cards are the final fallback — real production triage decisions commonly
// declare no committed_floors at all (D1 severity amplifier: without this
// fallback, every wave planned from such a decision yields zero specs, the
// livelock the empty-plan guard in cmd_loop_wave.go otherwise has to catch on
// every single wave). Each todo's Files is set to its own ID so PlanCycles
// treats every floor/package/card as independent work, spread across up to
// `count` lanes via its existing least-loaded partitioning; a todo id
// repeated within or between sources collapses to one todo (same underlying
// package). Malformed decisionJSON returns an explicit error and zero specs —
// callers must WARN and fall back to the sequential path rather than guess an
// unscoped launch.
func PlanFromTriage(decisionJSON []byte, cardPackages []string, count int) ([]CycleSpec, error) {
	var decision triageDecision
	if len(decisionJSON) > 0 {
		if err := json.Unmarshal(decisionJSON, &decision); err != nil {
			return nil, fmt.Errorf("fleet: parse triage-decision.json: %w", err)
		}
	}
	ids := decision.CommittedFloors
	if len(ids) == 0 {
		ids = cardPackages
	}
	if len(ids) == 0 {
		for _, card := range decision.TopN {
			if card.ID != "" {
				ids = append(ids, card.ID)
			}
		}
	}
	seen := make(map[string]bool, len(ids))
	todos := make([]Todo, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		todos = append(todos, Todo{ID: id, Files: []string{id}})
	}
	specs, _ := PlanCycles(todos, count)
	return specs, nil
}
