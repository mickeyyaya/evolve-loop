package ship

import (
	"context"
	"slices"
	"testing"
)

// TestStageExplicitPaths_QuotePathDisabledOnGitReads is the behavioural
// (non-source-grep) proof: drive a real cycle ship through shipDirect and
// read the argv git was actually invoked with. Both classification reads
// must carry `-c core.quotePath=false` BEFORE the subcommand (git config
// args are only legal there), which is the zero-parsing fix for the common
// non-ASCII case.
func TestStageExplicitPaths_QuotePathDisabledOnGitReads(t *testing.T) {
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
	cap := &porcelainCapture{
		porcelain: " M go/internal/phases/ship/gitops.go\n",
		ignored:   []string{"go/internal/phases/ship/gitops.go"}, // force the probe to run
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

// configArgIndex returns the index of the `-c` whose value is want, or -1.
func configArgIndex(args []string, want string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" && args[i+1] == want {
			return i
		}
	}
	return -1
}
