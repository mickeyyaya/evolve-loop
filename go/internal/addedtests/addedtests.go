// Package addedtests is the ONE derivation of "which test packages does this
// tree ADD, and under which build tags must each run" — the seed the ship's
// repo-contract added-test backstop has used since #612, now shared with the
// build floor so a red added test is found while the builder still owns the
// tree. Cycle 1679 (2026-09-14): an added `//go:build acs` package was
// invisible to the floor's default-context run (no GoFiles in that context)
// and first executed by the ship gate, where its red cost repair rounds the
// builder could not apply.
package addedtests

import (
	"bufio"
	"go/build"
	"go/build/constraint"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

// Group is one set of added test packages that run under the same build tags
// (Tags empty = the default build context).
type Group struct {
	Tags     []string
	Packages []string
}

// Groups collects the ADDED test files among files (repo-relative paths; only
// go/**/_test.go count — a modified test's package is the importer backstop's
// job) and groups their packages by the build tags each file declares, so a
// tag-guarded reproducer runs under its own tags. Excluded lists the added
// tests no tag selection can run on this host (requires_tmux, or a constraint
// too wide to enumerate) — callers name them rather than silently skipping.
func Groups(root string, files []changedpkgs.ChangedFile) (groups []Group, excluded []string, err error) {
	packagesByTags := map[string]map[string]bool{}
	tagsByKey := map[string][]string{}
	for _, f := range files {
		path := f.Path
		if !f.Added || !strings.HasPrefix(path, "go/") || !strings.HasSuffix(path, "_test.go") {
			continue
		}
		tags, runnable, matchErr := BuildTags(filepath.Join(root, path))
		if matchErr != nil {
			return nil, nil, matchErr
		}
		if !runnable {
			excluded = append(excluded, path)
			continue
		}
		pkg := "./" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(path, "go/")))
		key := strings.Join(tags, ",")
		if packagesByTags[key] == nil {
			packagesByTags[key] = map[string]bool{}
			tagsByKey[key] = tags
		}
		packagesByTags[key][pkg] = true
	}
	keys := make([]string, 0, len(packagesByTags))
	for key := range packagesByTags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := Group{Tags: tagsByKey[key]}
		for pkg := range packagesByTags[key] {
			group.Packages = append(group.Packages, pkg)
		}
		sort.Strings(group.Packages)
		groups = append(groups, group)
	}
	return groups, excluded, nil
}

// BuildTags reports the smallest tag selection under which the file at path
// builds: (nil, true) when it builds in the default context; (_, false) when
// no selection runs it here (requires_tmux, or more than 12 tags — the
// exhaustive search's ceiling; upgrade to a constraint solver past it).
func BuildTags(path string) (tags []string, runnable bool, err error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	match, err := build.Default.MatchFile(dir, name)
	if err != nil || match {
		return nil, match, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	var expr constraint.Expr
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if constraint.IsGoBuild(line) {
			expr, err = constraint.Parse(line)
			break
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		return nil, false, scanErr
	}
	if err != nil || expr == nil {
		return nil, false, err
	}
	tagSet := map[string]bool{}
	collectTags(expr, tagSet)
	if tagSet["requires_tmux"] || len(tagSet) > 12 {
		return nil, false, nil
	}
	all := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		all = append(all, tag)
	}
	sort.Strings(all)
	for mask := 1; mask < 1<<len(all); mask++ {
		candidate := make([]string, 0, len(all))
		for i, tag := range all {
			if mask&(1<<i) != 0 {
				candidate = append(candidate, tag)
			}
		}
		ctx := build.Default
		ctx.BuildTags = candidate
		if match, matchErr := ctx.MatchFile(dir, name); matchErr != nil {
			return nil, false, matchErr
		} else if match {
			return candidate, true, nil
		}
	}
	return nil, false, nil
}

func collectTags(expr constraint.Expr, tags map[string]bool) {
	switch x := expr.(type) {
	case *constraint.TagExpr:
		tags[x.Tag] = true
	case *constraint.NotExpr:
		collectTags(x.X, tags)
	case *constraint.AndExpr:
		collectTags(x.X, tags)
		collectTags(x.Y, tags)
	case *constraint.OrExpr:
		collectTags(x.X, tags)
		collectTags(x.Y, tags)
	}
}
