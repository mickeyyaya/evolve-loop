package wtcheckpoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type refEntry struct{ name, oid, tree string }

func (r refEntry) worktree() string {
	rest := strings.TrimPrefix(r.name, refPrefix)
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}

func (r refEntry) stamp() string {
	return r.name[strings.LastIndex(r.name, "/")+1:]
}

func worktreeRefs(ctx context.Context, g gitexec.Git, name string) ([]refEntry, error) {
	return listRefs(ctx, g, refPrefix+name+"/")
}

func listRefs(ctx context.Context, g gitexec.Git, pattern string) ([]refEntry, error) {
	out, err := g.Output(ctx, "for-each-ref", "--format=%(refname) %(objectname) %(tree)", pattern)
	if err != nil {
		return nil, err
	}
	var refs []refEntry
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 {
			return nil, fmt.Errorf("wtcheckpoint: %s is not a checkpoint commit: %q", f[0], line)
		}
		refs = append(refs, refEntry{name: f[0], oid: f[1], tree: f[2]})
	}
	return refs, nil
}

func deleteRefs(ctx context.Context, g gitexec.Git, refs []refEntry) ([]string, error) {
	var deleted []string
	for _, r := range refs {
		if err := g.Run(ctx, "update-ref", "-d", r.name, r.oid); err != nil {
			return deleted, err
		}
		deleted = append(deleted, r.name)
	}
	return deleted, nil
}

func beyondNewest(refs []refEntry, keep int) []refEntry {
	if keep <= 0 || len(refs) <= keep {
		return nil
	}
	return refs[:len(refs)-keep]
}

func groupByWorktree(refs []refEntry) map[string][]refEntry {
	groups := map[string][]refEntry{}
	for _, r := range refs {
		groups[r.worktree()] = append(groups[r.worktree()], r)
	}
	return groups
}
