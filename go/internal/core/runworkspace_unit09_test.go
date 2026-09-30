package core

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestRunWorkspacePath_ProjectsPathsLayout(t *testing.T) {
	t.Parallel()
	for _, cycle := range []int{0, 7, 1425} {
		got := RunWorkspacePath("/r", cycle)
		if got != paths.RunWorkspace("/r", cycle) || got != filepath.Join("/r", ".evolve", "runs", "cycle-"+strconv.Itoa(cycle)) {
			t.Fatalf("cycle %d: RunWorkspacePath = %q, paths.RunWorkspace = %q", cycle, got, paths.RunWorkspace("/r", cycle))
		}
	}
}
