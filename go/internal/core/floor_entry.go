package core

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func FloorEntryRun(judge FloorJudge) func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
	return func(ctx context.Context, in gatecontract.Input, _ gatecontract.Ports) (gatecontract.Verdict, error) {
		if err := readableRoot(in.Root); err != nil {
			return gatecontract.Verdict{}, err
		}
		lines, err := judge(ctx, floorEntryInput(in))
		if err != nil {
			return gatecontract.Verdict{}, err
		}
		return gatecontract.Verdict{Reds: lines}, nil
	}
}

func floorEntryInput(in gatecontract.Input) ReviewInput {
	return ReviewInput{Worktree: in.Root, ProjectRoot: in.Root, WorktreeBaseSHA: in.Base}
}

func readableRoot(root string) error {
	info, err := os.Stat(root)
	switch {
	case err != nil:
		return fmt.Errorf("the tree cannot be read: %w", err)
	case !info.IsDir():
		return fmt.Errorf("the tree %s is not a directory", root)
	}
	return nil
}
