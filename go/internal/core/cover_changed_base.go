package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var coverChangedBaseRunner = baseFailingTests

func laneRedLines(ctx context.Context, in ReviewInput, pkgs []string, out string) []string {
	failed := failedTestNames(out)
	baseRed, laneOnly := coverChangedBaseRed(ctx, in, pkgs, failed)
	if len(failed) > 0 && len(laneOnly) == 0 {
		fmt.Fprintf(in.errOut(), "[build-floor] NOTE cover-changed: each failing test is red on the base %s too, so the floor does not refuse it: %s\n", floorBase(in), strings.Join(baseRed, ", "))
		return nil
	}
	lines := []string{floorFailureDiagnostic(out)}
	if len(baseRed) > 0 {
		lines = append(lines, "red on the lane and green on the base: "+strings.Join(laneOnly, ", "), "information: red on the base too, not refused: "+strings.Join(baseRed, ", "))
	}
	return lines
}

func coverChangedBaseRed(ctx context.Context, in ReviewInput, pkgs, failed []string) (baseRed, laneOnly []string) {
	if len(failed) == 0 {
		return nil, nil
	}
	red, err := coverChangedBaseRunner(ctx, in, pkgs, failed)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN cover-changed could not run the failing tests on the base %s (%v); the floor refuses each failure\n", floorBase(in), err)
		return nil, failed
	}
	for _, n := range failed {
		if slices.Contains(red, n) {
			baseRed = append(baseRed, n)
		} else {
			laneOnly = append(laneOnly, n)
		}
	}
	return baseRed, laneOnly
}

func baseFailingTests(ctx context.Context, in ReviewInput, pkgs, names []string) ([]string, error) {
	dir, err := os.MkdirTemp("", "buildfloor-base-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tree, err := exportTree(ctx, in.Worktree, floorBase(in), dir)
	if err != nil {
		return nil, err
	}
	cmd := sysexec.Command(ctx, "go", baseTestArgs(pkgs, names)...)
	cmd.Dir = codequality.ModuleDir(tree)
	cmd.Env = lanebudget.Env(ipcenv.GoTestEnv())
	out, _ := cmd.CombinedOutput()
	return failedTestNames(string(out)), nil
}

func exportTree(ctx context.Context, worktree, rev, dir string) (string, error) {
	tree := filepath.Join(dir, "tree")
	git := gitexec.Git{Dir: worktree, Exec: sysexec.WithEnv(gitRunner, append(ipcenv.Scrub(os.Environ()), "GIT_INDEX_FILE="+filepath.Join(dir, "index")))}
	for _, args := range [][]string{{"read-tree", rev}, {"checkout-index", "--all", "--prefix=" + tree + "/"}} {
		if err := git.Run(ctx, args...); err != nil {
			return "", err
		}
	}
	return tree, nil
}

func baseTestArgs(pkgs, names []string) []string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	args := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-tags", strings.Join(covchanged.DefaultTags, ","), "-run", "^(" + strings.Join(quoted, "|") + ")$"}
	return lanebudget.Args(append(args, pkgs...))
}

func failedTestNames(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		rest, ok := strings.CutPrefix(line, "--- FAIL: ")
		if !ok {
			continue
		}
		if name, _, _ := strings.Cut(rest, " "); !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
