package core

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// TestRunWorkspacePath_ProjectsPathsLayout is the consumer pin (ADR-0103 unit
// 09): core's RunWorkspacePath is a projection of paths.RunWorkspace, the one
// home of the <root>/.evolve/runs/cycle-<N> layout — a divergence is red, not
// silent.
func TestRunWorkspacePath_ProjectsPathsLayout(t *testing.T) {
	t.Parallel()
	for _, cycle := range []int{0, 7, 1425} {
		got := RunWorkspacePath("/r", cycle)
		if got != paths.RunWorkspace("/r", cycle) || got != filepath.Join("/r", ".evolve", "runs", "cycle-"+strconv.Itoa(cycle)) {
			t.Fatalf("cycle %d: RunWorkspacePath = %q, paths.RunWorkspace = %q", cycle, got, paths.RunWorkspace("/r", cycle))
		}
	}
}
