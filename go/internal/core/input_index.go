package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type indexTarget struct {
	projectRoot, workspace, runID string
	cycle                         int
	specInputs                    []string
}

func writeInputIndex(at indexTarget, phase Phase, itemIDs []string, warn io.Writer) {
	record, err := cycleRecord(at, itemIDs)
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
	if recorded, ok := inputindex.ReadRecord(at.projectRoot, at.cycle); ok && recorded.RunID == at.runID {
		return recorded, nil
	}
	record := inputindex.Record{Cycle: at.cycle, RunID: at.runID, ItemIDs: itemIDs}
	return record, inputindex.WriteRecord(at.projectRoot, record)
}
