package fleet

// quota_test.go — fleet-s3-guards AC3 (cycle 467): RED-first contract for the
// quota-aware wave Count shrink. QuotaAwareCount does not exist yet; this
// file fails to COMPILE until Builder adds it — that compile failure IS the
// RED evidence.
//
// Contract: QuotaAwareCount(count int, benched map[string]string, warn
// io.Writer) int. benched maps a REQUIRED CLI family to its active bench
// reason/pattern — the caller intersects clihealth.Store.Active() (the quota
// bench SSOT, scout Key Finding 4 / H2: the bridge already writes benches on
// every quota classification, no new probing) with the families the wave
// needs, and passes only the relevant entries. Each benched required family
// shrinks the effective lane count (a benched family cannot absorb its share
// of concurrent lanes), clamped to a minimum of 1 so the loop always retains
// its sequential fallback. Every shrink WARNs on `warn`, naming the family
// AND the bench reason so the operator sees WHY capacity dropped.

import (
	"bytes"
	"strings"
	"testing"
)

// TestQuotaAwareCount_BenchedFamilyShrinksAndWarns (AC3): one benched
// required family at count=4 must yield a SMALLER effective count (still
// >=1), and the WARN must name both the family and its bench reason. Gaming
// fake it kills: a helper that returns count unchanged, or warns without
// shrinking.
func TestQuotaAwareCount_BenchedFamilyShrinksAndWarns(t *testing.T) {
	var warn bytes.Buffer
	got := QuotaAwareCount(4, map[string]string{"codex": "rate_limit"}, 1, &warn)
	if got >= 4 {
		t.Errorf("QuotaAwareCount(4, one benched family) = %d, want < 4 (benched capacity must shrink the wave)", got)
	}
	if got < 1 {
		t.Errorf("QuotaAwareCount(4, one benched family) = %d, want >= 1 (min-1 floor)", got)
	}
	out := warn.String()
	if !strings.Contains(out, "codex") {
		t.Errorf("WARN must name the benched family %q; got: %q", "codex", out)
	}
	if !strings.Contains(out, "rate_limit") {
		t.Errorf("WARN must name the bench reason %q; got: %q", "rate_limit", out)
	}
}

// TestQuotaAwareCount_MinOneClamp (AC3, edge/boundary): the shrink NEVER
// drops the count below 1, however many families are benched — the wave path
// degrades to a single lane, it never plans zero work.
func TestQuotaAwareCount_MinOneClamp(t *testing.T) {
	benched := map[string]string{
		"codex":  "rate_limit",
		"gemini": "quota_exhausted",
		"agy":    "rate_limit",
	}
	cases := []struct {
		name  string
		count int
	}{
		{"more-benches-than-count", 2},
		{"count-already-one", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var warn bytes.Buffer
			if got := QuotaAwareCount(tc.count, benched, 1, &warn); got != 1 {
				t.Errorf("QuotaAwareCount(%d, 3 benched families) = %d, want exactly 1 (min-1 clamp)", tc.count, got)
			}
		})
	}
}

// TestQuotaAwareCount_MinLanesFloorHoldsCapacity is the fleet.min_lanes fix
// (2026-07-03): the operator's asserted concurrent-lane budget survives a
// transient CLI-family bench. count=2, one benched family (codex), floor=2 →
// the wave STAYS at 2 (2 lanes on the healthy claude family) instead of the
// pre-fix collapse to 1. The WARN still names the benched family + reason so
// the operator sees the bench, but reports capacity HELD, not shrunk. Gaming
// this: return count-1 (ignores the floor) fails; drop the WARN fails.
func TestQuotaAwareCount_MinLanesFloorHoldsCapacity(t *testing.T) {
	var warn bytes.Buffer
	got := QuotaAwareCount(2, map[string]string{"codex": "rate_limit"}, 2, &warn)
	if got != 2 {
		t.Errorf("QuotaAwareCount(2, {codex}, minLanes=2) = %d, want 2 (floor holds capacity through the bench)", got)
	}
	out := warn.String()
	if !strings.Contains(out, "codex") || !strings.Contains(out, "rate_limit") {
		t.Errorf("WARN must still name the benched family + reason; got: %q", out)
	}
	if !strings.Contains(out, "min_lanes") {
		t.Errorf("WARN must report the floor held capacity (name fleet.min_lanes); got: %q", out)
	}
}

// TestQuotaAwareCount_MinLanesClampedToCount: a floor above the configured
// count is meaningless and must clamp DOWN to count, never inflate the wave.
// count=2, floor=5, one benched family → the wave stays at 2 (clamped floor),
// never 5.
func TestQuotaAwareCount_MinLanesClampedToCount(t *testing.T) {
	var warn bytes.Buffer
	if got := QuotaAwareCount(2, map[string]string{"codex": "rate_limit"}, 5, &warn); got != 2 {
		t.Errorf("QuotaAwareCount(2, {codex}, minLanes=5) = %d, want 2 (floor clamped to count, not inflated)", got)
	}
}

// TestQuotaAwareCount_MinLanesFloorPartialShrink: floor between 1 and count
// lets SOME shrink happen but stops at the floor. count=4, floor=2, three
// benched families → 4→3→2→(held at 2), never 1.
func TestQuotaAwareCount_MinLanesFloorPartialShrink(t *testing.T) {
	var warn bytes.Buffer
	benched := map[string]string{"codex": "rate_limit", "gemini": "quota", "agy": "rate_limit"}
	if got := QuotaAwareCount(4, benched, 2, &warn); got != 2 {
		t.Errorf("QuotaAwareCount(4, 3 benched, minLanes=2) = %d, want 2 (shrinks to the floor, not below)", got)
	}
	// Once the floor is reached, the remaining benches must emit the floor-held
	// WARN (not a "wave count N -> M" shrink line) — pins that the floor-held
	// branch is actually reachable and fires for a floored family.
	out := warn.String()
	if !strings.Contains(out, "capacity held at 2 by fleet.min_lanes floor") {
		t.Errorf("a bench absorbed by the floor must WARN 'capacity held at 2 by fleet.min_lanes floor'; got:\n%s", out)
	}
}

// TestQuotaAwareCount_NoBenchesNoShrinkNoWarn (AC3, negative/no-false-
// positive): with zero active benches the count passes through UNCHANGED and
// nothing is written to warn — a healthy fleet must never be throttled or
// spammed.
func TestQuotaAwareCount_NoBenchesNoShrinkNoWarn(t *testing.T) {
	for name, benched := range map[string]map[string]string{"empty-map": {}, "nil-map": nil} {
		t.Run(name, func(t *testing.T) {
			var warn bytes.Buffer
			if got := QuotaAwareCount(4, benched, 1, &warn); got != 4 {
				t.Errorf("QuotaAwareCount(4, no benches) = %d, want 4 unchanged", got)
			}
			if warn.Len() != 0 {
				t.Errorf("QuotaAwareCount(4, no benches) wrote a WARN (%q), want silence", warn.String())
			}
		})
	}
}
