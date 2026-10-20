package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
)

const floorListRemedy = "restore the base line; a lowered floor or a dropped package lands only as an operator change of its own"

func floorListRegressions(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	moduleDir := codequality.ModuleDir(in.Worktree)
	enforceBase, enforceTree, enforceRead := listPair(ctx, in, moduleDir, ".apicover-enforce")
	strictBase, strictTree, strictRead := listPair(ctx, in, moduleDir, ".cover-strict")
	if !enforceRead || !strictRead {
		return nil
	}
	gone := func(pkg string) bool { return packageRemoved(moduleDir, pkg) }
	out := droppedEnforceLines(enforceBase, enforceTree, gone)
	return append(out, weakenedFloors(strictBase, strictTree, gone)...)
}

func listPair(ctx context.Context, in ReviewInput, moduleDir, name string) (base, tree string, ok bool) {
	path := filepath.Join(moduleDir, name)
	atBase, err := listAtBase(ctx, in.Worktree, floorBase(in), path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN read %s at the base %s: %v; the floor list check did not run\n", name, floorBase(in), err)
		return "", "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN read %s: %v\n", name, err)
	}
	return atBase, string(raw), true
}

func listLines(list string) map[string]bool {
	lines := map[string]bool{}
	for _, line := range strings.Split(list, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			lines[enforcePattern(line)] = true
		}
	}
	return lines
}

func enforcePattern(line string) string {
	return "./" + strings.TrimPrefix(strings.TrimSuffix(line, "/..."), "./")
}

func packageRemoved(moduleDir, pkg string) bool {
	dir := strings.TrimSuffix(strings.TrimPrefix(pkg, "./"), "/...")
	_, err := os.Stat(filepath.Join(moduleDir, filepath.FromSlash(dir)))
	return errors.Is(err, fs.ErrNotExist)
}

func droppedEnforceLines(base, tree string, gone func(string) bool) []string {
	kept := listLines(tree)
	var out []string
	for _, line := range slices.Sorted(maps.Keys(listLines(base))) {
		if !kept[line] && !gone(line) {
			out = append(out, "floor list: go/.apicover-enforce drops "+line+", which the base enforces; "+floorListRemedy)
		}
	}
	return out
}

func weakenedFloors(base, tree string, gone func(string) bool) []string {
	baseFloors, err := coverstrict.ParseFloors([]byte(base))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN the base go/.cover-strict does not parse (%v); the floor regression check skipped it\n", err)
		return nil
	}
	treeFloors, err := coverstrict.ParseFloors([]byte(tree))
	if err != nil {
		return []string{"floor list: go/.cover-strict does not parse (" + err.Error() + "), so its floors cannot bind; fix the line"}
	}
	treeMin := map[string]float64{}
	for _, f := range treeFloors {
		if prev, seen := treeMin[f.Pkg]; !seen || f.Min > prev {
			treeMin[f.Pkg] = f.Min
		}
	}
	var out []string
	for _, f := range baseFloors {
		got, kept := treeMin[f.Pkg]
		switch {
		case !kept && gone(f.Pkg):
		case !kept:
			out = append(out, fmt.Sprintf("floor list: go/.cover-strict drops the floor of ./%s (%s at the base); %s", f.Pkg, percent(f.Min), floorListRemedy))
		case got < f.Min:
			out = append(out, fmt.Sprintf("floor list: go/.cover-strict lowers the floor of ./%s from %s to %s; %s", f.Pkg, percent(f.Min), percent(got), floorListRemedy))
		}
	}
	return out
}

func percent(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
