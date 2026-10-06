package wtcheckpoint

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

type PruneReason string

const (
	PruneRetention PruneReason = "retention"
	PruneLanded    PruneReason = "landed"
)

type PruneOptions struct {
	Keep   int
	Landed bool
}

type Pruned struct {
	Ref    string      `json:"ref"`
	Reason PruneReason `json:"reason"`
}

func Prune(ctx context.Context, h Hub, o PruneOptions) ([]Pruned, error) {
	g := h.git()
	refs, err := listRefs(ctx, g, refPrefix)
	if err != nil {
		return nil, err
	}
	if o.Landed {
		if _, err := g.Output(ctx, "rev-parse", "--verify", plane.OriginMainRef+"^{commit}"); err != nil {
			return nil, fmt.Errorf("wtcheckpoint: --landed needs %s to judge against: %w", plane.OriginMainRef, err)
		}
	}
	doomed, err := pruneCandidates(ctx, g, groupByWorktree(refs), o)
	if err != nil {
		return nil, err
	}
	var pruned []Pruned
	for _, d := range doomed {
		if _, err := deleteRefs(ctx, g, []refEntry{d.ref}); err != nil {
			return pruned, err
		}
		pruned = append(pruned, Pruned{Ref: d.ref.name, Reason: d.reason})
	}
	return pruned, nil
}

type candidate struct {
	ref    refEntry
	reason PruneReason
}

func pruneCandidates(ctx context.Context, g gitexec.Git, groups map[string][]refEntry, o PruneOptions) ([]candidate, error) {
	var out []candidate
	for _, name := range slices.Sorted(maps.Keys(groups)) {
		group := groups[name]
		old := beyondNewest(group, o.Keep)
		for _, r := range old {
			out = append(out, candidate{ref: r, reason: PruneRetention})
		}
		if !o.Landed {
			continue
		}
		for _, r := range group[len(old):] {
			landed, err := hasLanded(ctx, g, r)
			if err != nil {
				return nil, err
			}
			if landed {
				out = append(out, candidate{ref: r, reason: PruneLanded})
			}
		}
	}
	return out, nil
}

func hasLanded(ctx context.Context, g gitexec.Git, r refEntry) (bool, error) {
	changed, err := changedPaths(ctx, g, r.oid+"~2", r.oid)
	if err != nil || len(changed) == 0 {
		return false, err
	}
	differs, err := changedPaths(ctx, g, plane.OriginMainRef, r.oid)
	if err != nil {
		return false, err
	}
	notOnMain := map[string]bool{}
	for _, p := range differs {
		notOnMain[p] = true
	}
	return !slices.ContainsFunc(changed, func(p string) bool { return notOnMain[p] }), nil
}

func changedPaths(ctx context.Context, g gitexec.Git, from, to string) ([]string, error) {
	out, err := g.Output(ctx, "diff", "--name-only", "-z", "--no-renames", from, to)
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == 0 }), nil
}
