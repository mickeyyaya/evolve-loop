package dossier

import (
	"context"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// orphanPair matches a cycle-<N>.json or cycle-<N>.md basename.
var orphanPair = regexp.MustCompile(`(^|/)cycle-(\d+)\.(json|md)$`)

// SweepResult reports what a SweepOrphans pass did, keyed by cycle number.
type SweepResult struct {
	Recommitted []int         // complete pairs successfully committed (sorted)
	Skipped     []int         // incomplete pairs (a lone .json or .md) left untouched (sorted)
	Failed      map[int]error // pairs whose recommit failed; the batch continued past each
}

// SweepOrphans recommits every complete dirty cycle-N.{json,md} pair in g's
// working tree (untracked or modified) and skips half pairs. A per-pair failure
// is logged to logw and recorded in Failed; only a failure to enumerate the
// tree returns an error.
func SweepOrphans(g gitexec.Git, logw io.Writer) (SweepResult, error) {
	res := SweepResult{Failed: map[int]error{}}

	dirty, err := g.DirtyPaths(context.Background())
	if err != nil {
		return res, fmt.Errorf("dossier: sweep: enumerate tree: %w", err)
	}

	type pair struct{ json, md string }
	pairs := map[int]*pair{}
	for _, p := range dirty {
		m := orphanPair.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		n, convErr := strconv.Atoi(m[2])
		if convErr != nil {
			continue
		}
		pp := pairs[n]
		if pp == nil {
			pp = &pair{}
			pairs[n] = pp
		}
		if m[3] == "json" {
			pp.json = p
		} else {
			pp.md = p
		}
	}

	cycles := make([]int, 0, len(pairs))
	for n := range pairs {
		cycles = append(cycles, n)
	}
	sort.Ints(cycles)

	for _, n := range cycles {
		pp := pairs[n]
		if pp.json == "" || pp.md == "" {
			res.Skipped = append(res.Skipped, n)
			continue
		}
		base := strings.TrimSuffix(pp.json, ".json")
		if err := commitPairGit(g, base); err != nil {
			res.Failed[n] = err
			fmt.Fprintf(logw, "[dossier-sweep] ERROR cycle %d (%s): recommit failed: %v\n", n, path.Dir(pp.json), err)
			continue
		}
		res.Recommitted = append(res.Recommitted, n)
	}
	return res, nil
}
