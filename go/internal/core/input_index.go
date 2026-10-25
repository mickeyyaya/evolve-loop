package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func writeInputIndex(projectRoot, workspace string, cycle int, phase Phase, warn io.Writer) {
	limit := policy.LoadOrDefaults(filepath.Join(projectRoot, ".evolve", "policy.json"), warn).IsolationLineageMax()
	idx, err := inputindex.Build(inputindex.Request{ProjectRoot: projectRoot, Workspace: workspace, Cycle: cycle, Phase: string(phase), LineageMax: limit})
	if err == nil {
		err = inputindex.Write(workspace, idx)
	}
	if err != nil {
		_ = os.Remove(inputindex.Path(workspace))
		fmt.Fprintf(warn, "[orchestrator] WARN input index for %s: %v (the cell gets no index and no lineage grants)\n", phase, err)
	}
}
