package hostsentinel

import (
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeDiskFloor      signalcenter.Code = "HOST_SENTINEL_DISK_FLOOR"
	CodeDiskWarn       signalcenter.Code = "HOST_SENTINEL_DISK_WARN"
	CodeGoCacheOver    signalcenter.Code = "HOST_GOCACHE_OVER"
	CodeServersHigh    signalcenter.Code = "HOST_ORPHAN_SERVERS_HIGH"
	CodeOrphansHigh    signalcenter.Code = "HOST_ORPHANS_HIGH"
	CodeReleased       signalcenter.Code = "HOST_RELEASED"
	CodeWouldRelease   signalcenter.Code = "HOST_WOULD_RELEASE"
	CodeWouldHold      signalcenter.Code = "HOST_WOULD_HOLD"
	CodeReleaseSkipped signalcenter.Code = "HOST_RELEASE_SKIPPED"
	CodeLaneHold       signalcenter.Code = "HOST_LANE_HOLD"
	CodeLaneHoldStuck  signalcenter.Code = "HOST_LANE_HOLD_STUCK"
	CodeBlind          signalcenter.Code = "HOST_SENTINEL_BLIND"
)

const origin = "hostsentinel"

const BlindAfter = 3

func init() {
	for _, c := range []struct {
		code signalcenter.Code
		doc  string
	}{
		{CodeDiskFloor, "the host sentinel read free disk under gc.disk_floor_gb during a wave; WARN. Fields: fields.free_bytes, floor_bytes, warn_bytes, path"},
		{CodeDiskWarn, "the host sentinel read free disk under gc.disk_warn_gb and over the floor during a wave; INFO. Fields: fields.free_bytes, warn_bytes, path"},
		{CodeGoCacheOver, "the host sentinel measured the go build cache over gc.go_cache_max_gb; WARN. Fields: fields.cache_bytes, limit_bytes"},
		{CodeServersHigh, "the host sentinel counted more owner-dead tmux servers than gc.sentinel.orphan_servers_max; WARN. Fields: fields.count, limit"},
		{CodeOrphansHigh, "the host sentinel counted more orphan processes of finished cycles than gc.sentinel.orphan_processes_max; WARN. Fields: fields.count, limit"},
		{CodeReleased, "the mid-wave release ran the go cache trim and the pipeline temp sweep; INFO. Fields: fields.freed_bytes (free disk delta), cache_bytes (re-measured)"},
		{CodeWouldRelease, "at gc.sentinel.release.stage shadow, the host sentinel records the mid-wave release that enforce starts; INFO. Fields: fields.scopes, free_bytes, cache_bytes"},
		{CodeWouldHold, "at gc.sentinel.hold.stage shadow, the host sentinel records the hold on new lanes that enforce sets or clears; INFO. Fields: fields.would_hold (true or false), free_bytes, floor_bytes"},
		{CodeReleaseSkipped, "the host sentinel skipped a mid-wave release; INFO. Fields: fields.reason (gc_lease_held or the failure)"},
		{CodeLaneHold, "the host sentinel holds new lane starts because free disk is under gc.disk_floor_gb; running lanes continue; WARN. Fields: fields.free_bytes, floor_bytes"},
		{CodeLaneHoldStuck, "the hold on new lanes lasted longer than gc.sentinel.hold_max_s and stays on; INCIDENT. Fields: fields.held_s, free_bytes"},
		{CodeBlind, "the host sentinel could not read the host for " + strconv.Itoa(BlindAfter) + " samples in a row; WARN. Fields: fields.reason"},
	} {
		signalcenter.RegisterCode(signalcenter.ModuleHost, c.code, c.doc)
	}
}

type Publish func(signalcenter.Event)

type Mode struct {
	EnforceHold    bool
	EnforceRelease bool
}

func (p Publish) emit(code signalcenter.Code, kind signalcenter.Kind, sev signalcenter.Severity, reason string, fields map[string]string) {
	if p == nil {
		return
	}
	p(signalcenter.Event{Module: signalcenter.ModuleHost, Origin: origin, Kind: kind, Severity: sev, Code: code, Reason: reason, Fields: maps.Clone(fields)})
}

func (p Publish) Announce(d Decision, s Sample, c Config, m Mode) {
	for _, f := range d.Findings {
		p.emit(f.Code, signalcenter.KindHostSentinel, severityOf(f.Code), f.Reason, f.Fields)
	}
	switch {
	case d.HoldChanged && d.Hold && m.EnforceHold:
		p.LaneHold(s.FreeBytes, c.FloorBytes)
	case d.HoldChanged && !m.EnforceHold:
		p.WouldHold(d.Hold, s.FreeBytes, c.FloorBytes)
	}
	if d.Stuck && m.EnforceHold {
		p.LaneHoldStuck(d.HeldFor, s.FreeBytes)
	}
	if d.Release && !m.EnforceRelease {
		p.WouldRelease(Scopes, s.FreeBytes, s.CacheBytes)
	}
}

var Scopes = []string{"go_cache", "pipeline_temp"}

func severityOf(code signalcenter.Code) signalcenter.Severity {
	if code == CodeDiskWarn {
		return signalcenter.SeverityInfo
	}
	return signalcenter.SeverityWarn
}

func (p Publish) WouldHold(would bool, free, floor uint64) {
	p.emit(CodeWouldHold, signalcenter.KindHostSentinel, signalcenter.SeverityInfo, "shadow: enforce would change the hold on new lanes",
		map[string]string{"would_hold": strconv.FormatBool(would), "free_bytes": utoa(free), "floor_bytes": utoa(floor)})
}

func (p Publish) WouldRelease(scopes []string, free uint64, cache int64) {
	p.emit(CodeWouldRelease, signalcenter.KindHostSentinel, signalcenter.SeverityInfo, "shadow: enforce would run the mid-wave release",
		map[string]string{"scopes": strings.Join(scopes, ","), "free_bytes": utoa(free), "cache_bytes": itoa(cache)})
}

func (p Publish) LaneHold(free, floor uint64) {
	p.emit(CodeLaneHold, signalcenter.KindLoopWave, signalcenter.SeverityWarn, "free disk is under the floor, so no new lane starts",
		map[string]string{"free_bytes": utoa(free), "floor_bytes": utoa(floor)})
}

func (p Publish) LaneHoldStuck(held time.Duration, free uint64) {
	p.emit(CodeLaneHoldStuck, signalcenter.KindLoopWave, signalcenter.SeverityIncident, "the hold on new lanes lasted past gc.sentinel.hold_max_s and stays on",
		map[string]string{"held_s": strconv.Itoa(int(held.Seconds())), "free_bytes": utoa(free)})
}

func (p Publish) Released(freed, cache int64) {
	p.emit(CodeReleased, signalcenter.KindHostSentinel, signalcenter.SeverityInfo, "the mid-wave release ran",
		map[string]string{"freed_bytes": itoa(freed), "cache_bytes": itoa(cache)})
}

func (p Publish) ReleaseSkipped(reason string) {
	p.emit(CodeReleaseSkipped, signalcenter.KindHostSentinel, signalcenter.SeverityInfo, "the mid-wave release was skipped: "+reason,
		map[string]string{"reason": reason})
}

func (p Publish) Blind(reason string) {
	p.emit(CodeBlind, signalcenter.KindHostSentinel, signalcenter.SeverityWarn, "the host sentinel cannot read the host: "+reason,
		map[string]string{"reason": reason})
}
