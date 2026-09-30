package dashboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

var dossierFile = regexp.MustCompile(`^cycle-(\d+)\.json$`)

const trendPointCap = 120

func dossierFileName(cycle int) string { return fmt.Sprintf("cycle-%d.json", cycle) }

type dossierCache struct {
	mu      sync.Mutex
	entries map[string]cachedDossier
	parses  int
}

type cachedDossier struct {
	modTime time.Time
	size    int64
	d       *dossier.Dossier
}

func newDossierCache() *dossierCache {
	return &dossierCache{entries: map[string]cachedDossier{}}
}

func (c *dossierCache) load(path string, info os.FileInfo) (*dossier.Dossier, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if hit, ok := c.entries[path]; ok && hit.modTime.Equal(info.ModTime()) && hit.size == info.Size() {
		return hit.d, nil
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c.parses++
	d, err := dossier.ParseJSON(buf)
	if err != nil {
		return nil, err
	}
	c.entries[path] = cachedDossier{modTime: info.ModTime(), size: info.Size(), d: d}
	return d, nil
}

func (c *dossierCache) loadCycle(root string, cycle int) (*dossier.Dossier, error) {
	path := filepath.Join(dossier.CyclesDir(root), dossierFileName(cycle))
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dossier %s: %w", dossierFileName(cycle), err)
	}
	d, err := c.load(path, info)
	if err != nil {
		return nil, fmt.Errorf("dossier %s: %w", dossierFileName(cycle), err)
	}
	return d, nil
}

type history struct {
	Trend        Trend
	Fingerprints []FingerprintStat
	Dossiers     map[int]*dossier.Dossier
	Warnings     []string
}

func readHistory(root string, cache *dossierCache) history {
	h := history{Dossiers: map[int]*dossier.Dossier{}}
	dir := dossier.CyclesDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			h.Warnings = append(h.Warnings, fmt.Sprintf("dossiers %s: %v", dir, err))
		}
		return h
	}
	for _, e := range entries {
		m := dossierFile.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		cycle, _ := strconv.Atoi(m[1])
		info, err := e.Info()
		if err != nil {
			continue
		}
		d, err := cache.load(filepath.Join(dir, e.Name()), info)
		if err != nil {
			h.Warnings = append(h.Warnings, fmt.Sprintf("dossier %s: %v", e.Name(), err))
			continue
		}
		h.Dossiers[cycle] = d
	}
	cycles := sortedCycles(h.Dossiers)
	h.Trend = computeTrend(cycles, h.Dossiers)
	h.Fingerprints = computeFingerprints(cycles, h.Dossiers)
	return h
}

func sortedCycles(ds map[int]*dossier.Dossier) []int {
	out := make([]int, 0, len(ds))
	for c := range ds {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

func shipped(d *dossier.Dossier) bool {
	switch d.FinalVerdict {
	case dossier.VerdictPass:
		return true
	case dossier.VerdictWarn:
		return d.CommitSHA != ""
	}
	return false
}

func computeTrend(cycles []int, ds map[int]*dossier.Dossier) Trend {
	var t Trend
	points := make([]TrendPoint, 0, len(cycles))
	for _, c := range cycles {
		d := ds[c]
		s := shipped(d)
		points = append(points, TrendPoint{Cycle: c, Verdict: d.FinalVerdict, Shipped: s})
		t.Closed++
		if s {
			t.Shipped++
		}
	}
	t.ShipRateAll = shipRateOfLast(points, len(points))
	t.ShipRateLast20 = shipRateOfLast(points, 20)
	t.ShipRateLast50 = shipRateOfLast(points, 50)
	t.ShipStreak, t.LastZeroShipRun = shipStreak(points)
	if len(points) > trendPointCap {
		points = points[len(points)-trendPointCap:]
	}
	t.Points = points
	return t
}

func shipStreak(points []TrendPoint) (int, *ZeroShipRun) {
	i := len(points) - 1
	for i >= 0 && points[i].Shipped {
		i--
	}
	streak, end := len(points)-1-i, i
	for i >= 0 && !points[i].Shipped {
		i--
	}
	if end < 0 {
		return streak, nil
	}
	return streak, &ZeroShipRun{FirstCycle: points[i+1].Cycle, LastCycle: points[end].Cycle, Length: end - i}
}

func shipRateOfLast(points []TrendPoint, n int) float64 {
	if n > len(points) {
		n = len(points)
	}
	if n == 0 {
		return 0
	}
	shippedN := 0
	for _, p := range points[len(points)-n:] {
		if p.Shipped {
			shippedN++
		}
	}
	return float64(shippedN) / float64(n)
}

func computeFingerprints(cycles []int, ds map[int]*dossier.Dossier) []FingerprintStat {
	stats := map[string]*FingerprintStat{}
	lastShippedCycle := 0
	var order []string
	for _, c := range cycles {
		d := ds[c]
		if shipped(d) {
			lastShippedCycle = c
			continue
		}
		if d.Failure == nil || d.Failure.Fingerprint == "" {
			continue
		}
		fp := d.Failure.Fingerprint
		s, ok := stats[fp]
		if !ok {
			s = &FingerprintStat{Fingerprint: fp, PreClass: d.Failure.PreClass, FirstCycle: c}
			stats[fp] = s
			order = append(order, fp)
		} else if lastShippedCycle > s.LastCycle {
			s.Regressed = true
		}
		s.Count++
		s.LastCycle = c
		if len(d.Failure.Reasons) > 0 {
			s.Reason = d.Failure.Reasons[0]
		}
	}
	out := make([]FingerprintStat, 0, len(order))
	for _, fp := range order {
		out = append(out, *stats[fp])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastCycle > out[j].LastCycle })
	return out
}
