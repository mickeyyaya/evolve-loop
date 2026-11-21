package hostsentinel_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/hostsentinel"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recorder struct{ events []signalcenter.Event }

func (r *recorder) publish(e signalcenter.Event) { r.events = append(r.events, e) }

func (r *recorder) codes() []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range r.events {
		out = append(out, e.Code)
	}
	return out
}

func equalCodes(got []signalcenter.Code, want ...signalcenter.Code) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestEveryCodeIsRegisteredUnderTheHostModule(t *testing.T) {
	t.Parallel()
	for _, c := range []signalcenter.Code{
		hostsentinel.CodeDiskFloor, hostsentinel.CodeDiskWarn, hostsentinel.CodeGoCacheOver, hostsentinel.CodeServersHigh,
		hostsentinel.CodeOrphansHigh, hostsentinel.CodeReleased, hostsentinel.CodeWouldRelease, hostsentinel.CodeWouldHold,
		hostsentinel.CodeReleaseSkipped, hostsentinel.CodeLaneHold, hostsentinel.CodeLaneHoldStuck, hostsentinel.CodeBlind,
	} {
		if owner, ok := signalcenter.IsRegistered(c); !ok || owner != signalcenter.ModuleHost {
			t.Errorf("%s: registered %v owner %q, want the host module", c, ok, owner)
		}
	}
}

func TestAnnounce_FindingsGoOutWithTheirSeverityAndKind(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	s := hostsentinel.Sample{FreeBytes: 15 * gb, Path: "/p", CacheBytes: int64(30 * gb), CacheRead: true}
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	hostsentinel.Publish(rec.publish).Announce(d, s, testConfig(), hostsentinel.Mode{EnforceRelease: true})
	if !equalCodes(rec.codes(), hostsentinel.CodeDiskWarn, hostsentinel.CodeGoCacheOver) {
		t.Fatalf("codes = %v", rec.codes())
	}
	if e := rec.events[0]; e.Severity != signalcenter.SeverityInfo || e.Kind != signalcenter.KindHostSentinel || e.Module != signalcenter.ModuleHost {
		t.Errorf("warn event = %+v", e)
	}
	if rec.events[1].Severity != signalcenter.SeverityWarn {
		t.Errorf("cache event severity = %v", rec.events[1].Severity)
	}
}

func TestAnnounce_ShadowRecordsTheReleaseAndHoldAndRunsNothing(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	s := hostsentinel.Sample{FreeBytes: 8 * gb, Path: "/p", CacheBytes: int64(41 * gb), CacheRead: true}
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	hostsentinel.Publish(rec.publish).Announce(d, s, testConfig(), hostsentinel.Mode{})
	if !equalCodes(rec.codes(), hostsentinel.CodeDiskFloor, hostsentinel.CodeGoCacheOver, hostsentinel.CodeWouldHold, hostsentinel.CodeWouldRelease) {
		t.Fatalf("codes = %v", rec.codes())
	}
	hold, release := rec.events[2], rec.events[3]
	if hold.Fields["would_hold"] != "true" || hold.Fields["free_bytes"] != "8000000000" || hold.Fields["floor_bytes"] != "10000000000" {
		t.Errorf("hold fields = %v", hold.Fields)
	}
	if release.Fields["scopes"] != "go_cache,pipeline_temp" || release.Fields["cache_bytes"] != "41000000000" {
		t.Errorf("release fields = %v", release.Fields)
	}
}

func TestAnnounce_ShadowRecordsTheClearOfTheHold(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	cfg := testConfig()
	low := healthy()
	low.FreeBytes = 5 * gb
	_, st := hostsentinel.Decide(low, cfg, hostsentinel.State{}, t0)
	d, _ := hostsentinel.Decide(healthy(), cfg, st, t0.Add(time.Minute))
	hostsentinel.Publish(rec.publish).Announce(d, healthy(), cfg, hostsentinel.Mode{EnforceRelease: true})
	if !equalCodes(rec.codes(), hostsentinel.CodeWouldHold) || rec.events[0].Fields["would_hold"] != "false" {
		t.Fatalf("events = %+v", rec.events)
	}
}

func TestAnnounce_EnforceSignalsTheLaneHoldAndTheStuckIncident(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	cfg := testConfig()
	low := healthy()
	low.FreeBytes = 5 * gb
	d, st := hostsentinel.Decide(low, cfg, hostsentinel.State{}, t0)
	pub := hostsentinel.Publish(rec.publish)
	pub.Announce(d, low, cfg, hostsentinel.Mode{EnforceHold: true, EnforceRelease: true})
	d, _ = hostsentinel.Decide(low, cfg, st, t0.Add(cfg.HoldMax))
	pub.Announce(d, low, cfg, hostsentinel.Mode{EnforceHold: true, EnforceRelease: true})
	if !equalCodes(rec.codes(), hostsentinel.CodeDiskFloor, hostsentinel.CodeLaneHold, hostsentinel.CodeDiskFloor, hostsentinel.CodeLaneHoldStuck) {
		t.Fatalf("codes = %v", rec.codes())
	}
	hold, stuck := rec.events[1], rec.events[3]
	if hold.Kind != signalcenter.KindLoopWave || hold.Severity != signalcenter.SeverityWarn || hold.Fields["free_bytes"] != "5000000000" {
		t.Errorf("hold event = %+v", hold)
	}
	if stuck.Kind != signalcenter.KindLoopWave || stuck.Severity != signalcenter.SeverityIncident || stuck.Fields["held_s"] != "7200" {
		t.Errorf("stuck event = %+v", stuck)
	}
}

func TestPublish_ReleasedSkippedAndBlindCarryTheirFields(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	pub := hostsentinel.Publish(rec.publish)
	pub.Released(20_000_000_000, 53_000_000_000)
	pub.ReleaseSkipped("gc_lease_held")
	pub.Blind("statfs failed")
	if !equalCodes(rec.codes(), hostsentinel.CodeReleased, hostsentinel.CodeReleaseSkipped, hostsentinel.CodeBlind) {
		t.Fatalf("codes = %v", rec.codes())
	}
	if f := rec.events[0].Fields; f["freed_bytes"] != "20000000000" || f["cache_bytes"] != "53000000000" {
		t.Errorf("released fields = %v", f)
	}
	if rec.events[1].Fields["reason"] != "gc_lease_held" || rec.events[2].Fields["reason"] != "statfs failed" {
		t.Errorf("reasons = %v %v", rec.events[1].Fields, rec.events[2].Fields)
	}
}

func TestPublish_ANilPublisherDropsTheEvent(t *testing.T) {
	t.Parallel()
	var pub hostsentinel.Publish
	s := hostsentinel.Sample{FreeBytes: 5 * gb, Path: "/p", CacheBytes: int64(41 * gb), CacheRead: true}
	d, _ := hostsentinel.Decide(s, testConfig(), hostsentinel.State{}, t0)
	if len(d.Findings) == 0 || !d.Hold || !d.Release {
		t.Fatalf("setup: the decision has nothing to publish: %+v", d)
	}
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		pub.Blind("x")
		pub.Announce(d, s, testConfig(), hostsentinel.Mode{EnforceHold: true})
		return nil
	}()
	if panicked != nil {
		t.Fatalf("a nil publisher panicked: %v", panicked)
	}
}

func TestPublish_TheHoldAndReleaseRecordersCarryTheirFieldsAndKinds(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	pub := hostsentinel.Publish(rec.publish)
	pub.WouldHold(false, 25*gb, 10*gb)
	pub.WouldRelease(hostsentinel.Scopes, 8*gb, 41_000_000_000)
	pub.LaneHold(8*gb, 10*gb)
	pub.LaneHoldStuck(2*time.Hour, 8*gb)
	if !equalCodes(rec.codes(), hostsentinel.CodeWouldHold, hostsentinel.CodeWouldRelease, hostsentinel.CodeLaneHold, hostsentinel.CodeLaneHoldStuck) {
		t.Fatalf("codes = %v", rec.codes())
	}
	if f := rec.events[0].Fields; f["would_hold"] != "false" || f["free_bytes"] != "25000000000" || f["floor_bytes"] != "10000000000" {
		t.Errorf("would hold fields = %v", f)
	}
	if f := rec.events[1].Fields; f["scopes"] != "go_cache,pipeline_temp" || f["cache_bytes"] != "41000000000" {
		t.Errorf("would release fields = %v", f)
	}
	if e := rec.events[2]; e.Kind != signalcenter.KindLoopWave || e.Severity != signalcenter.SeverityWarn {
		t.Errorf("lane hold = %+v", e)
	}
	if e := rec.events[3]; e.Fields["held_s"] != "7200" || e.Severity != signalcenter.SeverityIncident {
		t.Errorf("lane hold stuck = %+v", e)
	}
}

func TestScopes_NameTheTwoStepsOfTheCacheScope(t *testing.T) {
	t.Parallel()
	if len(hostsentinel.Scopes) != 2 || hostsentinel.Scopes[0] != "go_cache" || hostsentinel.Scopes[1] != "pipeline_temp" {
		t.Fatalf("scopes = %v, want the go cache trim and the pipeline temp sweep", hostsentinel.Scopes)
	}
}

func TestFinding_CarriesItsCodeReasonAndFields(t *testing.T) {
	t.Parallel()
	d, _ := hostsentinel.Decide(hostsentinel.Sample{FreeBytes: 8 * gb, Path: "/p"}, testConfig(), hostsentinel.State{}, t0)
	var f hostsentinel.Finding = d.Findings[0]
	if f.Code != hostsentinel.CodeDiskFloor || f.Reason == "" || f.Fields["path"] != "/p" {
		t.Fatalf("finding = %+v", f)
	}
}
