package ship

import (
	"context"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

func verifyManualStage(ctx context.Context, opts *Options, res *RunResult) error {
	if opts.DryRun {
		return previewStagedSet(ctx, opts, res)
	}
	return verifyManualConfirm(ctx, opts, res)
}

func previewStagedSet(ctx context.Context, opts *Options, res *RunResult) error {
	porcelain, err := captureGitOutput(ctx, opts, shipmanifest.RawPathRead("status", "--porcelain", "-uall")...)
	if err != nil {
		return err
	}
	set := shipmanifest.ChangedPaths(porcelain)
	if len(set) == 0 {
		res.Logs = append(res.Logs, "[ship] no changes in the staged set; nothing to ship")
		return errEmptyDiff
	}
	for _, p := range set {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] [DRY-RUN] staged set: %s", p))
	}
	return nil
}

type pathsKey struct{}

func WithPaths(ctx context.Context, named []string) context.Context {
	if len(named) == 0 {
		return ctx
	}
	return context.WithValue(ctx, pathsKey{}, named)
}

func pathsOf(ctx context.Context) []string {
	named, _ := ctx.Value(pathsKey{}).([]string)
	return named
}

func pathspec(ctx context.Context) []string {
	named := pathsOf(ctx)
	if len(named) == 0 {
		return nil
	}
	return append([]string{"--"}, named...)
}

func stageArgs(ctx context.Context) []string {
	return append([]string{"add", "-A"}, pathspec(ctx)...)
}

func scopeStatusRead(ctx context.Context, args []string) []string {
	if !slices.Contains(args, "status") || !slices.Contains(args, "--porcelain") || !slices.Contains(args, "-uall") {
		return args
	}
	return append(slices.Clone(args), pathspec(ctx)...)
}
