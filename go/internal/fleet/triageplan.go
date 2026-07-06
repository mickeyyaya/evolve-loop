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
		// Files is the card's declared repo footprint. When present it
		// populates the todo's Files so fleet.Partition clusters cards that
		// share a real file into ONE lane (safe: one cycle, one worktree)
		// instead of fabricating fictional-disjoint concurrent lanes off the
		// id-as-file placeholder. Absent → falls back to the id island.
		Files []string `json:"files"`
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
//
// A todo's Files carries its real repo footprint when the source declares one:
// top_n[] cards may declare files[], which populate Todo.Files so cards sharing
// a file collapse into ONE partition lane while disjoint cards still spread to
// `count`. committed_floors / cardPackages are bare string sources (a package
// path IS its footprint), so they keep the id-as-file island. Every source
// falls back to []string{id} when no files are declared — file-less work stays
// an independent island, preserving today's spread for those inputs exactly.
func PlanFromTriage(decisionJSON []byte, cardPackages []string, count int) ([]CycleSpec, error) {
	todos, err := TodosFromTriage(decisionJSON, cardPackages)
	if err != nil {
		return nil, err
	}
	specs, _ := PlanCycles(todos, count)
	return specs, nil
}

// TodosFromTriage parses a triage-decision.json (+ optional cardPackages
// fallback) into the disjoint-aware Todo backlog PlanFromTriage partitions.
// Exported so the rolling-pool dispatch path (cmd_loop_pool.go) rolls the SAME
// backlog through fleet.RunPool that the wave path partitions statically —
// single-sourcing the decision→todos parse across both schedulers.
func TodosFromTriage(decisionJSON []byte, cardPackages []string) ([]Todo, error) {
	var decision triageDecision
	if len(decisionJSON) > 0 {
		if err := json.Unmarshal(decisionJSON, &decision); err != nil {
			return nil, fmt.Errorf("fleet: parse triage-decision.json: %w", err)
		}
	}
	// sources preserves the historic precedence (committed_floors, then
	// cardPackages, then top_n cards) while letting a source carry declared
	// files. Only top_n cards can declare files; the string sources stay islands.
	type todoSource struct {
		id    string
		files []string
	}
	var sources []todoSource
	switch {
	case len(decision.CommittedFloors) > 0:
		for _, id := range decision.CommittedFloors {
			sources = append(sources, todoSource{id: id})
		}
	case len(cardPackages) > 0:
		for _, id := range cardPackages {
			sources = append(sources, todoSource{id: id})
		}
	default:
		for _, card := range decision.TopN {
			if card.ID != "" {
				sources = append(sources, todoSource{id: card.ID, files: card.Files})
			}
		}
	}
	seen := make(map[string]bool, len(sources))
	todos := make([]Todo, 0, len(sources))
	for _, src := range sources {
		if src.id == "" || seen[src.id] {
			continue
		}
		seen[src.id] = true
		files := src.files
		if len(files) == 0 {
			files = []string{src.id}
		}
		todos = append(todos, Todo{ID: src.id, Files: files})
	}
	return todos, nil
}
