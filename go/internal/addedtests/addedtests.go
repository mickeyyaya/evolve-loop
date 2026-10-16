// Package addedtests derives which test packages a tree adds and the build tags each must run under.
// The ship gate and the build floor share this one derivation.
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

type Group struct {
	Tags     []string
	Packages []string
}

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

const maxSearchTags = 12

func BuildTags(path string) (tags []string, runnable bool, err error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	match, err := build.Default.MatchFile(dir, name)
	if err != nil || match {
		return nil, match, err
	}
	expr, err := goBuildExpr(path)
	if err != nil || expr == nil {
		return nil, false, err
	}
	tagSet := map[string]bool{}
	collectTags(expr, tagSet)
	if tagSet["requires_tmux"] || len(tagSet) > maxSearchTags {
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
		if match, matchErr := matchFileWithContext(ctx, dir, name); matchErr != nil {
			return nil, false, matchErr
		} else if match {
			return candidate, true, nil
		}
	}
	return nil, false, nil
}

var matchFileWithContext = func(ctx build.Context, dir, name string) (bool, error) {
	return ctx.MatchFile(dir, name)
}

func goBuildExpr(path string) (constraint.Expr, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
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
		return nil, scanErr
	}
	return expr, err
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
