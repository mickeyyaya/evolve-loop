package core

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var ErrLaneRecordUnwritable = errors.New("the lane record cannot be written")

type indexTarget struct {
	projectRoot, workspace, runID, goalHash string
	cycle                                   int
	specInputs                              []string
}

func writeInputIndex(at indexTarget, phase Phase, itemIDs []string, warn io.Writer) error {
	record, err := cycleRecord(at, itemIDs)
	if err != nil && len(itemIDs) > 0 {
		return fmt.Errorf("%w for cycle %d: %w", ErrLaneRecordUnwritable, at.cycle, err)
	}
	var idx inputindex.Index
	if err == nil {
		pol := policy.LoadOrDefaults(filepath.Join(at.projectRoot, ".evolve", "policy.json"), warn)
		idx, err = inputindex.Build(inputindex.Request{ProjectRoot: at.projectRoot, Workspace: at.workspace, Cycle: at.cycle, Phase: string(phase),
			LineageMax: pol.IsolationLineageMax(), ItemIDs: record.ItemIDs, Context: contextPolicy(pol.IsolationContext(), phase, at.specInputs)})
	}
	if err == nil {
		err = inputindex.Write(at.workspace, idx)
	}
	workspace := at.workspace
	if err != nil {
		_ = os.Remove(inputindex.Path(workspace))
		fmt.Fprintf(warn, "[orchestrator] WARN input index for %s: %v (the cell gets no index and no lineage grants)\n", phase, err)
	}
	return nil
}

func contextPolicy(c policy.IsolationContextPolicy, phase Phase, inputs []string) inputindex.ContextPolicy {
	return inputindex.ContextPolicy{Stage: c.Stage, Budget: inputindex.Budget{L1Tokens: c.Budget.L1Tokens, L2Tokens: c.Budget.L2Tokens},
		Required: c.Required[string(phase)], Inputs: inputs}
}

func fleetScopeIDs(scope string) []string {
	var ids []string
	for _, id := range strings.Split(scope, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func cycleRecord(at indexTarget, itemIDs []string) (inputindex.Record, error) {
	if !filepath.IsAbs(at.projectRoot) {
		return inputindex.Record{}, fmt.Errorf("the project root %q is not absolute", at.projectRoot)
	}
	record := inputindex.Record{Cycle: at.cycle, RunID: at.runID, ItemIDs: itemIDs, GoalHash: at.goalHash}
	if recorded, ok := inputindex.ReadRecord(at.projectRoot, at.cycle); ok {
		if recorded.RunID == at.runID {
			return recorded, nil
		}
		record.RoutedIDs = recorded.RoutedIDs
		if len(recorded.ItemIDs) > 0 {
			record.ItemIDs = recorded.ItemIDs
			if recorded.GoalHash != "" {
				record.GoalHash = recorded.GoalHash
			}
		}
	}
	return record, inputindex.WriteRecord(at.projectRoot, record)
}
