package core

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
)

func GateClassifierFloorChecks(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	git := worktreeGit(ctx, in.Worktree)
	read := func(args ...string) (string, error) {
		out, err := git(args...)
		return string(out), err
	}
	input, err := gateclass.Read(read, floorBase(in))
	if err != nil {
		return []string{"gate classifier (" + string(gateclass.CodeI10Violation) + "): read the cycle diff: " + err.Error()}
	}
	input.Cycle = in.Cycle
	flags := gateclass.Classify(input)
	if len(flags) == 0 {
		return nil
	}
	in.Signals.Emit(gateclass.ViolationEvent(in.Cycle, in.RunID, string(PhaseBuild), flags))
	failures := make([]string, len(flags))
	for i, f := range flags {
		failures[i] = fmt.Sprintf("gate classifier (%s): %s — a loop cycle may not change a kernel surface or weaken a gate (llm-isolation.md §20.3; ship refuses this diff). Restore the change. If the task needs it, record that in build-report.md: the item is operator work.", gateclass.CodeI10Violation, f)
	}
	return failures
}
