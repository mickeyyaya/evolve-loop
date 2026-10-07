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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

const defaultMaxCycles = 40

var workspaceDir = regexp.MustCompile(`^cycle-(\d+)$`)

type collector struct {
	root        string
	cache       *dossierCache
	streams     *streamReader
	maxCycles   int
	operatorEnv map[string]string
}

func newCollector(root string) *collector {
	return &collector{root: root, cache: newDossierCache(), streams: newStreamReader(), maxCycles: defaultMaxCycles}
}

func Collect(root string, now time.Time) *Snapshot {
	snap, _ := newCollector(root).collect(now)
	return snap
}

func (c *collector) collect(now time.Time) (*Snapshot, map[int]*dossier.Dossier) {
	snap := &Snapshot{GeneratedAt: now, Root: c.root}
	var warns []string
	snap.Loop, warns = readLoop(c.root, now)
	snap.Warnings = append(snap.Warnings, warns...)
	runs, warns := readRunStatuses(c.root, now, snap.Loop)
	snap.Warnings = append(snap.Warnings, warns...)
	if own, ok := runs[snap.Loop.CycleID]; ok {
		snap.Loop = own
	}
	for _, run := range runs {
		if run.Running && (!snap.Loop.Running || run.CycleID > snap.Loop.CycleID) {
			snap.Loop = run
		}
	}
	snap.Loop = enrichLoopStatus(c.root, snap.Loop)
	snap.Queue, warns = readQueue(c.root, now)
	snap.Warnings = append(snap.Warnings, warns...)
	h := readHistory(c.root, c.cache)
	snap.Warnings = append(snap.Warnings, h.Warnings...)
	snap.Trend, snap.Fingerprints = h.Trend, h.Fingerprints

	ids, warn := c.renderedCycleIDs(h, runs)
	if warn != "" {
		snap.Warnings = append(snap.Warnings, warn)
	}
	snap.Cycles = make([]CycleSummary, 0, len(ids))
	mandatory, w := readMandatory(c.root, c.operatorEnv)
	snap.Warnings = append(snap.Warnings, w...)
	for _, id := range ids {
		cs, w := readCycle(c.root, id, h.Dossiers[id])
		snap.Warnings = append(snap.Warnings, w...)
		status := runs[id]
		status.BrakeEngaged = snap.Loop.BrakeEngaged
		cs = assignState(cs, status)
		cs.Plan, w = readPlan(mandatory, core.RunWorkspacePath(c.root, id), cs, status, c.streams)
		snap.Warnings = append(snap.Warnings, w...)
		snap.Cycles = append(snap.Cycles, cs)
	}
	snap.Trend.RoundHistogram = roundHistogram(snap.Cycles)
	return snap, h.Dossiers
}

func (c *collector) renderedCycleIDs(h history, runs map[int]LoopStatus) ([]int, string) {
	ids, warn := c.selectCycles(h)
	selected := map[int]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	for id, run := range runs {
		if run.Running && !selected[id] {
			ids = append(ids, id)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	return ids, warn
}

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

func runsDir(root string) string { return filepath.Join(paths.EvolveDirOf(root), "runs") }

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
