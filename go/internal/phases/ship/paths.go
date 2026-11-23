package ship

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"

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

func stageManualSet(ctx context.Context, opts *Options) (int, error) {
	if len(pathsOf(ctx)) == 0 {
		return opts.run(ctx, "git", stageArgs(ctx), io.Discard, opts.Stderr)
	}
	keep, err := stageablePaths(ctx, opts)
	if err != nil {
		return 0, err
	}
	if len(keep) == 0 {
		return 0, nil
	}
	return opts.run(ctx, "git", append([]string{"add", "-A", "--"}, keep...), io.Discard, opts.Stderr)
}

func stageablePaths(ctx context.Context, opts *Options) ([]string, error) {
	var keep []string
	for _, p := range pathsOf(ctx) {
		if _, err := os.Lstat(filepath.Join(opts.ProjectRoot, p)); err == nil {
			keep = append(keep, p)
			continue
		}
		known, err := captureGitOutput(ctx, opts, shipmanifest.RawPathRead("ls-files", "--cached", "--deleted", "--", p)...)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(known) != "" {
			keep = append(keep, p)
			continue
		}
		staged, err := captureGitOutput(ctx, opts, shipmanifest.RawPathRead("diff", "--cached", "--name-only", "--diff-filter=D", "--", p)...)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(staged) == "" {
			return nil, shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageVerifyClass,
				"ship: --paths names a path unknown to the index, HEAD and disk: "+p, "path", p)
		}
	}
	return keep, nil
}

func scopeStatusRead(ctx context.Context, args []string) []string {
	if !slices.Contains(args, "status") || !slices.Contains(args, "--porcelain") || !slices.Contains(args, "-uall") {
		return args
	}
	return append(slices.Clone(args), pathspec(ctx)...)
}
