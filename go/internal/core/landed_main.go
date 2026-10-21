package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

var boundaryOwnedPrefixes = []string{"knowledge-base/cycles/", ".evolve/inbox/"}

func originMainRef(ctx context.Context, git gitFn, dir string) (string, bool) {
	if url, code, err := git(ctx, dir, "config", "--get", "remote.origin.url"); err != nil || code != 0 || strings.TrimSpace(url) == "" {
		return "", false
	}
	out, code, err := git(ctx, dir, originHeadArgs...)
	return "origin/" + defaultBranchOf(out, code, err), true
}

func unprovenAhead(ctx context.Context, git gitFn, dir, remote, head string) ([]string, error) {
	out, code, err := git(ctx, dir, "rev-list", "--parents", remote+".."+head)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("rev-list --parents %s..%s: rc=%d: %v", remote, head, code, err)
	}
	var unproven []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		proven, err := provenAhead(ctx, git, dir, remote, fields[0], fields[1:])
		if err != nil {
			return nil, err
		}
		if !proven {
			unproven = append(unproven, fields[0])
		}
	}
	return unproven, nil
}

func provenAhead(ctx context.Context, git gitFn, dir, remote, sha string, parents []string) (bool, error) {
	if len(parents) > 1 {
		for _, parent := range parents[1:] {
			_, code, err := git(ctx, dir, "merge-base", "--is-ancestor", parent, remote)
			if err != nil {
				return false, fmt.Errorf("merge-base --is-ancestor %s %s: %w", parent, remote, err)
			}
			if code != 0 {
				return false, nil
			}
		}
		return true, nil
	}
	out, code, err := git(ctx, dir, "diff-tree", "-z", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil || code != 0 {
		return false, fmt.Errorf("diff-tree %s: rc=%d: %v", sha, code, err)
	}
	paths := strings.Split(strings.TrimRight(out, "\x00"), "\x00")
	return !slices.ContainsFunc(paths, notBoundaryOwned), nil
}

func notBoundaryOwned(path string) bool {
	return !slices.ContainsFunc(boundaryOwnedPrefixes, func(prefix string) bool { return strings.HasPrefix(path, prefix) })
}

func shortSHAs(shas []string) string {
	out := make([]string, len(shas))
	for i, sha := range shas {
		out[i] = sha[:min(12, len(sha))]
	}
	return strings.Join(out, ", ")
}

func refuseUnprovenAhead(ctx context.Context, git gitFn, dir, remote, head string) error {
	unproven, err := unprovenAhead(ctx, git, dir, remote, head)
	if err != nil {
		return err
	}
	if len(unproven) > 0 {
		return fmt.Errorf("origin does not hold %s, and no boundary step wrote it; land it or remove it from the plane main (cycle 1830)", shortSHAs(unproven))
	}
	return nil
}
