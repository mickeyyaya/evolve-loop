package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func baseAndTreeList(ctx context.Context, in ReviewInput, moduleDir, name string) []byte {
	path := filepath.Join(moduleDir, name)
	tree, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN read %s: %v\n", name, err)
	}
	base := floorBase(in)
	atBase, err := listAtBase(ctx, in.Worktree, base, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN read %s at the base %s: %v; the floor judges the tree's list alone\n", name, base, err)
		return tree
	}
	return mergedLines(string(tree), atBase)
}

func mergedLines(lists ...string) []byte {
	seen := map[string]bool{}
	var out strings.Builder
	for _, line := range strings.Split(strings.Join(lists, "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		out.WriteString(line + "\n")
	}
	return []byte(out.String())
}

func listAtBase(ctx context.Context, worktree, base, path string) (string, error) {
	rel, err := filepath.Rel(worktree, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("%s is outside the worktree %s", path, worktree)
	}
	git := gitexec.Git{Dir: worktree, Exec: gitRunner}
	out, _, code, err := git.Capture(ctx, "show", base+":"+filepath.ToSlash(rel))
	if err == nil && code == 0 {
		return out, nil
	}
	if _, stderr, code, err := git.Capture(ctx, "rev-parse", "--verify", "--quiet", base+"^{commit}"); err != nil || code != 0 {
		return "", errors.Join(err, fmt.Errorf("the base does not resolve to a commit (exit %d)%s", code, strings.TrimSpace(" "+stderr)))
	}
	return "", nil
}

func enforcedPackages(ctx context.Context, in ReviewInput, moduleDir string, pkgs []string) map[string]bool {
	enforced := map[string]bool{}
	for _, p := range ciparity.IntersectEnforced(pkgs, baseAndTreeList(ctx, in, moduleDir, ".apicover-enforce")) {
		enforced[p] = true
	}
	return enforced
}
