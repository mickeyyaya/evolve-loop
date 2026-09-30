package commitgate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/binaryguard"
	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func (o Options) refuseWhatNeverCommits(ctx context.Context, files []string, res *Result) int {
	if code := o.refuseBinaries(files, res); code != ExitPass {
		return code
	}
	return o.refuseAddedComments(ctx, files, res)
}

func (o Options) refuseBinaries(files []string, res *Result) int {
	offenders, err := binaryguard.Scan(o.RepoRoot, files, binaryguard.DefaultThresholdBytes)
	if err != nil {
		res.log("binary guard: %v", err)
		return ExitGitFatal
	}
	for _, off := range offenders {
		res.log("REJECTED: %s is a %d-byte compiled executable — never commit build artifacts (add it to .gitignore).", off.Path, off.Size)
	}
	if len(offenders) > 0 {
		return ExitFail
	}
	return ExitPass
}

func (o Options) refuseAddedComments(ctx context.Context, files []string, res *Result) int {
	paths, err := o.changeWithRenameSources(ctx, files)
	if err != nil {
		res.log("comment check: %v", err)
		return ExitGitFatal
	}
	added, err := commentaudit.AddedAcrossDiff(paths, commentaudit.ReadAtBase(o.git(ctx), "HEAD"), o.readOnDisk)
	if err != nil {
		res.log("comment check: %v", err)
		return ExitGitFatal
	}
	for _, c := range added {
		res.log("REJECTED: %s adds a comment: %s", c.File, c.Line)
	}
	if len(added) > 0 {
		res.log("code carries no comments: say it in a name, a test name or the package's design notes (docs/conventions/code-comments.md)")
		return ExitFail
	}
	return ExitPass
}

func (o Options) changeWithRenameSources(ctx context.Context, files []string) ([]string, error) {
	listed, err := o.changeListing(ctx)
	if err != nil {
		return nil, err
	}
	paths := slices.Clone(files)
	for _, path := range listed {
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (o Options) changeListing(ctx context.Context) ([]string, error) {
	out, stderr, code, err := sysexec.Capture(ctx, o.Runner, o.RepoRoot, "git", "diff", "--name-only", "--no-renames", "-z", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("the change could not be listed with rename sources: %w", err)
	}
	if code > 1 {
		return nil, fmt.Errorf("the change could not be listed with rename sources (rc=%d): %s", code, strings.TrimSpace(stderr))
	}
	var paths []string
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func (o Options) git(ctx context.Context) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		out, stderr, code, err := sysexec.Capture(ctx, o.Runner, o.RepoRoot, "git", args...)
		if err != nil {
			return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		if code != 0 {
			return nil, fmt.Errorf("git %s: rc=%d: %s", strings.Join(args, " "), code, strings.TrimSpace(stderr))
		}
		return []byte(out), nil
	}
}

func (o Options) readOnDisk(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(o.RepoRoot, path))
}
