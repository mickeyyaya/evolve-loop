package ship

import (
	"context"
	"slices"
	"testing"
)

func TestStageExplicitPaths_QuotePathDisabledOnGitReads(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain: " M go/internal/phases/ship/gitops.go\n",
		ignored:   []string{"go/internal/phases/ship/gitops.go"},
	}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	for _, sub := range []string{"status", "check-ignore"} {
		call := cap.gitCallWith(sub)
		if call == nil {
			t.Fatalf("ship never invoked `git %s`; calls=%v", sub, cap.calls)
		}
		args := call[1:]
		cfgAt := configArgIndex(args, "core.quotePath=false")
		if cfgAt < 0 {
			t.Errorf("`git %s` argv %v lacks `-c core.quotePath=false` — git C-quotes any non-ASCII path and ship then parses an escaped string that exists on no disk", sub, args)
			continue
		}
		subAt := slices.Index(args, sub)
		if subAt >= 0 && cfgAt > subAt {
			t.Errorf("`git %s` argv %v places `-c core.quotePath=false` (index %d) after the subcommand (index %d) — git only accepts config args before the subcommand", sub, args, cfgAt, subAt)
		}
	}
}

func configArgIndex(args []string, want string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" && args[i+1] == want {
			return i
		}
	}
	return -1
}
