package fleet

import (
	"encoding/json"
	"fmt"
)

type triageDecision struct {
	CommittedFloors []string `json:"committed_floors"`
	TopN            []struct {
		ID    string   `json:"id"`
		Files []string `json:"files"`
	} `json:"top_n"`
}

func PlanFromTriage(decisionJSON []byte, cardPackages []string, count int, routed RoutedFn) ([]CycleSpec, []string, error) {
	todos, refused, err := TodosFromTriage(decisionJSON, cardPackages, routed)
	if err != nil {
		return nil, nil, err
	}
	specs, _ := PlanCycles(todos, count)
	return specs, refused, nil
}

type RoutedFn func(id string) (routed bool, reason string)

func TodosFromTriage(decisionJSON []byte, cardPackages []string, routed RoutedFn) (todos []Todo, refused []string, err error) {
	var decision triageDecision
	if len(decisionJSON) > 0 {
		if err := json.Unmarshal(decisionJSON, &decision); err != nil {
			return nil, nil, fmt.Errorf("fleet: parse triage-decision.json: %w", err)
		}
	}
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
	todos = make([]Todo, 0, len(sources))
	for _, src := range sources {
		if src.id == "" || seen[src.id] {
			continue
		}
		seen[src.id] = true
		if routed != nil {
			if r, reason := routed(src.id); r {
				refused = append(refused, src.id+": "+reason)
				continue
			}
		}
		files := src.files
		if len(files) == 0 {
			files = []string{src.id}
		}
		todos = append(todos, Todo{ID: src.id, Files: files})
	}
	return todos, refused, nil
}
