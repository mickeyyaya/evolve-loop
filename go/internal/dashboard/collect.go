package dashboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// defaultMaxCycles bounds the cycle list the board renders. Every cycle with
// a run workspace on disk is always included; dossier-only history fills the
// remainder, newest first.
const defaultMaxCycles = 40

// workspaceDir matches a live run workspace name. `cycle-N.polluted-<stamp>`
// (a quarantined duplicate) and `archive/` are deliberately excluded.
var workspaceDir = regexp.MustCompile(`^cycle-(\d+)$`)

// collector holds the caches a repeated collect reuses between poll ticks.
type collector struct {
	root      string
	cache     *dossierCache
	maxCycles int
}

func newCollector(root string) *collector {
	return &collector{root: root, cache: newDossierCache(), maxCycles: defaultMaxCycles}
}

// Collect reads the project root once and returns the whole picture. It is the
// one-shot form (`evolve dashboard --snapshot`); the server keeps a collector so
// unchanged dossiers are not re-parsed on every tick.
func Collect(root string, now time.Time) *Snapshot {
	snap, _ := newCollector(root).collect(now)
	return snap
}

// collect builds the snapshot and returns the dossier map it was built from,
// so the detail handler can serve a cycle from the same epoch without a second
// scan of the corpus.
func (c *collector) collect(now time.Time) (*Snapshot, map[int]*dossier.Dossier) {
	snap := &Snapshot{GeneratedAt: now, Root: c.root}
	var warns []string
	snap.Loop, warns = readLoop(c.root, now)
	snap.Warnings = append(snap.Warnings, warns...)
	snap.Queue, warns = readQueue(c.root)
	snap.Warnings = append(snap.Warnings, warns...)
	h := readHistory(c.root, c.cache)
	snap.Warnings = append(snap.Warnings, h.Warnings...)
	snap.Trend, snap.Fingerprints = h.Trend, h.Fingerprints

	ids, warn := c.selectCycles(h)
	if warn != "" {
		snap.Warnings = append(snap.Warnings, warn)
	}
	snap.Cycles = make([]CycleSummary, 0, len(ids))
	for _, id := range ids {
		cs, w := readCycle(c.root, id, h.Dossiers[id])
		snap.Warnings = append(snap.Warnings, w...)
		snap.Cycles = append(snap.Cycles, assignState(cs, snap.Loop))
	}
	snap.Trend.RoundHistogram = roundHistogram(snap.Cycles)
	return snap, h.Dossiers
}

// selectCycles returns the cycle ids to render, newest first: every run
// workspace, then the newest dossier-only cycles up to maxCycles.
func (c *collector) selectCycles(h history) ([]int, string) {
	set := map[int]bool{}
	wsIDs, warn := workspaceCycles(c.root)
	for _, id := range wsIDs {
		set[id] = true
	}
	dossierIDs := sortedCycles(h.Dossiers)
	for i := len(dossierIDs) - 1; i >= 0 && len(set) < c.maxCycles; i-- {
		set[dossierIDs[i]] = true
	}
	ids := make([]int, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	if len(ids) > c.maxCycles {
		ids = ids[:c.maxCycles]
	}
	return ids, warn
}

// runsDir is <root>/.evolve/runs, the parent of every run workspace.
func runsDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "runs") }

// workspaceCycles lists the live run workspace ids under .evolve/runs/. A
// missing directory is the empty project; an unreadable one is reported — a
// silent nil would empty the board AND freeze the change fingerprint.
func workspaceCycles(root string) ([]int, string) {
	dir := runsDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ""
		}
		return nil, fmt.Sprintf("runs %s: %v", dir, err)
	}
	var ids []int
	for _, e := range entries {
		if m := workspaceDir.FindStringSubmatch(e.Name()); m != nil && e.IsDir() {
			id, _ := strconv.Atoi(m[1])
			ids = append(ids, id)
		}
	}
	return ids, ""
}

// roundHistogram buckets CLOSED cycles that still have a workspace by the
// number of audit rounds they took, with how many of each shipped — the
// convergence view of the repair loop. Dossiers do not record rounds, so the
// histogram is bounded to the workspaces still on disk.
func roundHistogram(cycles []CycleSummary) []RoundBucket {
	buckets := map[int]*RoundBucket{}
	for _, cs := range cycles {
		if !cs.HasWorkspace || !cs.HasDossier {
			continue
		}
		b, ok := buckets[cs.AuditRounds]
		if !ok {
			b = &RoundBucket{Rounds: cs.AuditRounds}
			buckets[cs.AuditRounds] = b
		}
		b.Cycles++
		if cs.State == StatePass || (cs.State == StateWarn && cs.CommitSHA != "") {
			b.Shipped++
		}
	}
	out := make([]RoundBucket, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rounds < out[j].Rounds })
	return out
}
