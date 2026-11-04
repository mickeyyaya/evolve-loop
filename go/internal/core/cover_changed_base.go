package core

import (
	"context"
	"fmt"
	"os"
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
	failed, unnamed := failedTests(out)
	if unnamed || len(failed) == 0 {
		return []string{floorFailureDiagnostic(out), "a failure without a test name (a build failure, a panic or a timeout) counts against the lane"}
	}
	baseRed, laneOnly := coverChangedBaseRed(ctx, in, pkgs, failed)
	if len(laneOnly) == 0 {
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
	tree, err := gitexec.Git{Dir: in.Worktree, Exec: gitRunner}.ExportTree(ctx, floorBase(in), dir)
	if err != nil {
		return nil, err
	}
	cmd := sysexec.Command(ctx, "go", baseTestArgs(pkgs, names)...)
	cmd.Dir = codequality.ModuleDir(tree)
	cmd.Env = lanebudget.Env(ipcenv.GoTestEnv())
	out, _ := cmd.CombinedOutput()
	red, _ := failedTests(string(out))
	return red, nil
}

func baseTestArgs(pkgs, keys []string) []string {
	quoted := make([]string, len(keys))
	for i, k := range keys {
		quoted[i] = regexp.QuoteMeta(k[strings.LastIndex(k, ".")+1:])
	}
	args := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-tags", strings.Join(covchanged.DefaultTags, ","), "-run", "^(" + strings.Join(quoted, "|") + ")$"}
	return lanebudget.Args(append(args, pkgs...))
}

func failedTests(out string) (keys []string, unnamed bool) {
	var pending []string
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "--- FAIL: "); ok {
			name, _, _ := strings.Cut(rest, " ")
			pending = append(pending, name)
			continue
		}
		unnamed = unnamed || strings.HasPrefix(line, "panic: ")
		rest, ok := strings.CutPrefix(line, "FAIL\t")
		if !ok {
			continue
		}
		fields := append(strings.Fields(rest), "")
		unnamed = unnamed || len(pending) == 0
		for _, name := range pending {
			if key := fields[0] + "." + name; !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
		pending = nil
	}
	slices.Sort(keys)
	return keys, unnamed || len(pending) > 0
}
