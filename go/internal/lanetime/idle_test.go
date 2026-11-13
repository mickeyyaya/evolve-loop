package lanetime

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func idleSamples(t *testing.T, lines ...string) []Sample {
	t.Helper()
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	samples, _, err := ReadSamples(path)
	if err != nil {
		t.Fatal(err)
	}
	return samples
}

func novAt(h int) time.Time { return time.Date(2026, 11, 13, h, 0, 0, 0, time.UTC) }

type orchEvent struct {
	ts, code, cause string
	live            int
}

func (e orchEvent) line() string {
	extra := ""
	if e.cause != "" {
		extra = `,"cause":"` + e.cause + `"`
	}
	return `{"ts":"2026-11-13T` + e.ts + `:00Z","code":"` + e.code + `","pid":7,"fields":{"live":"` + string(rune('0'+e.live)) + `","target":"2"` + extra + `}}`
}

func TestReadSamples_CarriesTheCodeAndTheCauseAndNamesAMissingIdleCauseUnknown(t *testing.T) {
	samples := idleSamples(t,
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_IDLE", live: 1, cause: ""}.line(),
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_UNDERFILLED", live: 1, cause: "run_cap"}.line(),
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_FILLED", live: 1, cause: ""}.line())

	if len(samples) != 3 || samples[0].Cause != "unknown" || samples[0].Code != "LOOP_ORCH_POOL_IDLE" {
		t.Fatalf("samples = %+v, want an idle line with no cause to read unknown", samples)
	}
	if samples[1].Cause != "run_cap" || samples[2].Cause != "" || samples[2].Code != "LOOP_ORCH_POOL_FILLED" {
		t.Errorf("samples = %+v, want run_cap kept and a non-idle line to carry no cause", samples)
	}
}

func TestIdleUse_AttributesFreeSlotsToTheCauseOfTheIdleEvent(t *testing.T) {
	samples := idleSamples(t,
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_UNDERFILLED", live: 0, cause: "run_cap"}.line(),
		orchEvent{ts: "11:00", code: "LOOP_ORCH_POOL_IDLE", live: 1, cause: ""}.line(),
		orchEvent{ts: "12:00", code: "LOOP_ORCH_POOL_FILLED", live: 0, cause: ""}.line())

	got := IdleUse(samples, time.Time{}, novAt(13))

	if len(got) != 2 || got["run_cap"] != 2 || got["unknown"] != 1 {
		t.Errorf("idle use = %v, want run_cap 2 and unknown 1: a non-idle line with live < target is not idle time", got)
	}
}

func TestIdleUse_NoSpanPastTheEndAndNoNegativeSpan(t *testing.T) {
	samples := idleSamples(t,
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_UNDERFILLED", live: 1, cause: "run_cap"}.line(),
		orchEvent{ts: "13:00", code: "LOOP_ORCH_POOL_IDLE", live: 0, cause: "bench"}.line())

	got := IdleUse(samples, time.Time{}, novAt(12))

	if len(got) != 1 || got["run_cap"] != 2 {
		t.Errorf("idle use = %v, want only run_cap 2: the sample after the end adds nothing", got)
	}
}

func TestIdleUse_AZeroLengthSpanAddsNoKey(t *testing.T) {
	samples := idleSamples(t,
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_IDLE", live: 0, cause: "bench"}.line(),
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_FILLED", live: 2, cause: ""}.line())

	if got := IdleUse(samples, time.Time{}, time.Time{}); len(got) != 0 {
		t.Errorf("idle use = %v, want no key for a zero-length span", got)
	}
	if got := IdleUse(nil, time.Time{}, time.Time{}); len(got) != 0 {
		t.Errorf("idle use of no sample = %v, want none", got)
	}
}

func TestIdleUse_AnIdleEventWithNoFreeSlotAddsNoKey(t *testing.T) {
	t.Parallel()
	samples := idleSamples(t,
		orchEvent{ts: "10:00", code: "LOOP_ORCH_POOL_IDLE", live: 2, cause: "bench"}.line(),
		orchEvent{ts: "11:00", code: "LOOP_ORCH_POOL_FILLED", live: 2}.line())

	got := IdleUse(samples, time.Time{}, time.Time{})

	if len(got) != 0 {
		t.Errorf("IdleUse = %v, want no key for an idle event with live equal to target", got)
	}
}
