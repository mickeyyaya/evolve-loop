package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

type floorAdded struct {
	groups []addedtests.Group
	module string
	pkgs   map[string]bool
}

func (in ReviewInput) withFloorAdded() ReviewInput {
	in.added = readFloorAdded(in)
	return in
}

func readFloorAdded(in ReviewInput) floorAdded {
	if in.Worktree == "" || in.WorktreeBaseSHA == "" {
		return floorAdded{}
	}
	files, ok := changedpkgs.ChangedFilesChecked(in.Worktree, in.WorktreeBaseSHA)
	if !ok {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN: the changed files of %s cannot be read, so a green retake clears a test that the cycle added; the ship backstop still stands\n", in.Worktree)
		return floorAdded{}
	}
	groups, excluded, err := addedtests.Groups(in.Worktree, files)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN: added-test grouping failed (%v) — tag-gated added packages not run at the floor; the ship backstop still stands\n", err)
		return floorAdded{}
	}
	for _, p := range excluded {
		fmt.Fprintf(in.errOut(), "[build-floor] EXCLUDED %s (requires_tmux or another build constraint unavailable on this host)\n", p)
	}
	module, err := covchanged.ModulePath(codequality.ModuleDir(in.Worktree))
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN: the module path cannot be read (%v), so a green retake clears a test that the cycle added; the ship backstop still stands\n", err)
		return floorAdded{groups: groups}
	}
	pkgs := map[string]bool{}
	for _, g := range groups {
		for _, p := range g.Packages {
			pkgs[p] = true
		}
	}
	return floorAdded{groups: groups, module: module, pkgs: pkgs}
}

func (a floorAdded) relative(importPath string) string {
	if rest, ok := strings.CutPrefix(importPath, a.module+"/"); ok && a.module != "" {
		return "./" + rest
	}
	return importPath
}

func (a floorAdded) newTestReds(first string, covered []string) []baseattr.Key {
	var reds []baseattr.Key
	for _, k := range baseattr.ParseGoTest(first).Keys {
		pkg := a.relative(k.Package)
		if a.pkgs[pkg] && slices.Contains(covered, pkg) {
			reds = append(reds, baseattr.Key{Package: pkg, Test: k.Test})
		}
	}
	return reds
}

func redPackages(reds []baseattr.Key) string {
	var pkgs []string
	for _, k := range reds {
		if !slices.Contains(pkgs, k.Package) {
			pkgs = append(pkgs, k.Package)
		}
	}
	return strings.Join(pkgs, ",")
}

func redTests(reds []baseattr.Key) []string {
	tests := make([]string, len(reds))
	for i, k := range reds {
		tests[i] = k.Test
	}
	return tests
}

const floorCorrectionPrefix = "[floor-correction] "

func attachCorrection(correction, out string) string {
	return floorCorrectionPrefix + strings.ReplaceAll(correction, "\n", " ") + "\n" + out
}

func splitCorrection(out string) (correction, rest string) {
	line, rest, found := strings.Cut(out, "\n")
	if !found || !strings.HasPrefix(line, floorCorrectionPrefix) {
		return "", out
	}
	return strings.TrimPrefix(line, floorCorrectionPrefix), rest
}

func joinCorrection(correction, rest string) string {
	if correction == "" {
		return rest
	}
	return correction + "\n" + rest
}

func capKeepingCorrection(out string, limit int) string {
	correction, rest := splitCorrection(out)
	if len(rest) > limit {
		rest = rest[:limit] + "…"
	}
	return joinCorrection(correction, rest)
}
