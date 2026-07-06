package dossier

// sweep.go — cycle-564 orphan recovery (task
// sweep-orphaned-dossier-pairs-and-harden-commit).
//
// commitPair is best-effort: when it failed (historically un-retried, on a
// transient index.lock), the freshly-written cycle-N.{json,md} pair stayed
// untracked in the main tree with no commit history. Nine recorded cycle
// failures and 35 live orphaned pairs (cycle-564 scout) are exactly this: a
// later phase's tree-diff guard picks the stray pair up as unexplained churn
// and hard-aborts the cycle. SweepOrphans mops those up — it re-detects every
// untracked, COMPLETE dossier pair and recommits it, so the main tree lands
// clean without the next phase ever tripping.

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

// orphanPair matches a dossier path's basename: cycle-<N>.json or cycle-<N>.md.
// Anchored on the basename (via the leading "(^|/)") so it never false-positives
// a look-alike deeper in a path segment.
var orphanPair = regexp.MustCompile(`(^|/)cycle-(\d+)\.(json|md)$`)

// SweepResult reports what a SweepOrphans pass did, keyed by cycle number.
type SweepResult struct {
	Recommitted []int         // complete pairs successfully committed (sorted)
	Skipped     []int         // incomplete pairs (a lone .json or .md) left untouched (sorted)
	Failed      map[int]error // pairs whose recommit failed; the batch continued past each
}

// SweepOrphans finds untracked cycle-N.{json,md} pairs in g's working tree and
// recommits every COMPLETE pair (both files present), scoped by pathspec. An
// incomplete pair — a partial write from a killed dossier writer — is Skipped,
// never force-committed as a half pair. A per-pair commit failure is logged
// loudly (with the cycle number and the underlying git error) to logw and
// recorded in Failed; the sweep continues past it rather than aborting the whole
// batch on one bad pair. The returned error is non-nil only for an unrecoverable
// failure to even enumerate the tree.
func SweepOrphans(g gitexec.Git, logw io.Writer) (SweepResult, error) {
	res := SweepResult{Failed: map[int]error{}}

	dirty, err := g.DirtyPaths(context.Background())
	if err != nil {
		return res, fmt.Errorf("dossier: sweep: enumerate tree: %w", err)
	}

	// Collect the untracked path per (cycle, extension). A path is a candidate
	// only if git reports it untracked — a tracked/committed dossier is not an
	// orphan. DirtyPaths already restricts to dirty+untracked entries.
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
		// Both members share a directory and the cycle-N stem; commitPairGit
		// rebuilds the .json/.md names from the base, so strip the extension.
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
