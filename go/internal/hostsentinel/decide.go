// Package hostsentinel decides what a host reading means for a running wave: which signals to emit, whether to hold new lanes and whether to run the mid-wave release.
// See docs/architecture/packages/internal-hostsentinel.md.
package hostsentinel

import (
	"maps"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Sample struct {
	FreeBytes  uint64
	Path       string
	CacheBytes int64
	CacheRead  bool
	Servers    int
	Orphans    int
	CountsRead bool
}

type Config struct {
	FloorBytes      uint64
	WarnBytes       uint64
	CacheLimitBytes int64
	ServersMax      int
	OrphansMax      int
	Repeat          time.Duration
	HoldMax         time.Duration
	NoProgressBytes int64
}

type mark struct {
	at   time.Time
	band int
}

type State struct {
	Held         bool
	heldSince    time.Time
	stuck        bool
	marks        map[signalcenter.Code]mark
	releasedAt   time.Time
	stalledCache int64
	stalledFree  uint64
}

type Finding struct {
	Code   signalcenter.Code
	Reason string
	Fields map[string]string
}

type Decision struct {
	Findings    []Finding
	Hold        bool
	HoldChanged bool
	Stuck       bool
	HeldFor     time.Duration
	Release     bool
}

func Decide(s Sample, c Config, st State, now time.Time) (Decision, State) {
	next := st.copy()
	var d Decision
	d.Findings = append(d.Findings, next.diskFindings(s, c, now)...)
	d.Findings = append(d.Findings, next.cacheFindings(s, c, now)...)
	d.Findings = append(d.Findings, next.countFindings(s, c, now)...)
	next.decideHold(&d, s, c, now)
	next.decideRelease(&d, s, c, now)
	return d, next
}

func (st State) copy() State {
	st.marks = maps.Clone(st.marks)
	if st.marks == nil {
		st.marks = map[signalcenter.Code]mark{}
	}
	return st
}

func (st State) DeferRelease() State {
	st = st.copy()
	st.releasedAt = time.Time{}
	return st
}

func (st State) AfterRelease(c Config, freed int64, after Sample) State {
	st = st.copy()
	if freed >= c.NoProgressBytes {
		st.stalledCache, st.stalledFree = 0, 0
		return st
	}
	st.stalledFree = after.FreeBytes
	if after.CacheRead {
		st.stalledCache = after.CacheBytes
	}
	return st
}

func (st *State) due(code signalcenter.Code, band int, now time.Time, repeat time.Duration) bool {
	last, seen := st.marks[code]
	if seen && band <= last.band && now.Sub(last.at) < repeat {
		return false
	}
	st.marks[code] = mark{at: now, band: band}
	return true
}

func bandOf(second bool) int {
	if second {
		return 2
	}
	return 1
}

func utoa(v uint64) string { return strconv.FormatUint(v, 10) }

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func (st *State) diskFindings(s Sample, c Config, now time.Time) []Finding {
	switch {
	case c.FloorBytes > 0 && s.FreeBytes < c.FloorBytes:
		if !st.due(CodeDiskFloor, bandOf(s.FreeBytes <= c.FloorBytes/2), now, c.Repeat) {
			return nil
		}
		return []Finding{{Code: CodeDiskFloor, Reason: "free disk is under the floor", Fields: map[string]string{
			"free_bytes": utoa(s.FreeBytes), "floor_bytes": utoa(c.FloorBytes), "warn_bytes": utoa(c.WarnBytes), "path": s.Path}}}
	case c.WarnBytes > 0 && s.FreeBytes < c.WarnBytes:
		if !st.due(CodeDiskWarn, 1, now, c.Repeat) {
			return nil
		}
		return []Finding{{Code: CodeDiskWarn, Reason: "free disk is under the warn line", Fields: map[string]string{
			"free_bytes": utoa(s.FreeBytes), "warn_bytes": utoa(c.WarnBytes), "path": s.Path}}}
	}
	return nil
}

func (st *State) cacheFindings(s Sample, c Config, now time.Time) []Finding {
	if !s.CacheRead || c.CacheLimitBytes <= 0 || s.CacheBytes <= c.CacheLimitBytes {
		return nil
	}
	if !st.due(CodeGoCacheOver, bandOf(s.CacheBytes >= 2*c.CacheLimitBytes), now, c.Repeat) {
		return nil
	}
	return []Finding{{Code: CodeGoCacheOver, Reason: "the go build cache is over its cap", Fields: map[string]string{
		"cache_bytes": itoa(s.CacheBytes), "limit_bytes": itoa(c.CacheLimitBytes)}}}
}

func (st *State) countFindings(s Sample, c Config, now time.Time) []Finding {
	if !s.CountsRead {
		return nil
	}
	var out []Finding
	for _, r := range []struct {
		code         signalcenter.Code
		count, limit int
		reason       string
	}{
		{CodeServersHigh, s.Servers, c.ServersMax, "owner-dead tmux servers are over the limit"},
		{CodeOrphansHigh, s.Orphans, c.OrphansMax, "orphan processes of finished cycles are over the limit"},
	} {
		if r.limit <= 0 || r.count <= r.limit || !st.due(r.code, bandOf(r.count >= 2*r.limit), now, c.Repeat) {
			continue
		}
		out = append(out, Finding{Code: r.code, Reason: r.reason, Fields: map[string]string{
			"count": strconv.Itoa(r.count), "limit": strconv.Itoa(r.limit)}})
	}
	return out
}

func (st *State) decideHold(d *Decision, s Sample, c Config, now time.Time) {
	was := st.Held
	switch {
	case c.FloorBytes > 0 && s.FreeBytes < c.FloorBytes:
		st.Held = true
	case s.FreeBytes >= c.WarnBytes:
		st.Held = false
	}
	d.Hold, d.HoldChanged = st.Held, st.Held != was
	switch {
	case st.Held && !was:
		st.heldSince, st.stuck = now, false
	case !st.Held:
		st.heldSince, st.stuck = time.Time{}, false
	case !st.stuck && c.HoldMax > 0 && now.Sub(st.heldSince) >= c.HoldMax:
		st.stuck = true
		d.Stuck, d.HeldFor = true, now.Sub(st.heldSince)
	}
}

func (st *State) clearRecoveredBackoff(s Sample, c Config) {
	if c.WarnBytes > 0 && s.FreeBytes >= c.WarnBytes {
		st.stalledFree = 0
	}
	if s.CacheRead && c.CacheLimitBytes > 0 && s.CacheBytes <= c.CacheLimitBytes {
		st.stalledCache = 0
	}
}

func (st *State) decideRelease(d *Decision, s Sample, c Config, now time.Time) {
	st.clearRecoveredBackoff(s, c)
	diskLow := c.WarnBytes > 0 && s.FreeBytes < c.WarnBytes && (st.stalledFree == 0 || s.FreeBytes < st.stalledFree)
	cacheOver := s.CacheRead && c.CacheLimitBytes > 0 && s.CacheBytes > c.CacheLimitBytes && (st.stalledCache == 0 || s.CacheBytes > st.stalledCache)
	cooled := st.releasedAt.IsZero() || now.Sub(st.releasedAt) >= c.Repeat
	if (diskLow || cacheOver) && cooled {
		d.Release = true
		st.releasedAt = now
	}
}
