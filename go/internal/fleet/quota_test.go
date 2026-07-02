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
	got := QuotaAwareCount(4, map[string]string{"codex": "rate_limit"}, &warn)
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
			if got := QuotaAwareCount(tc.count, benched, &warn); got != 1 {
				t.Errorf("QuotaAwareCount(%d, 3 benched families) = %d, want exactly 1 (min-1 clamp)", tc.count, got)
			}
		})
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
			if got := QuotaAwareCount(4, benched, &warn); got != 4 {
				t.Errorf("QuotaAwareCount(4, no benches) = %d, want 4 unchanged", got)
			}
			if warn.Len() != 0 {
				t.Errorf("QuotaAwareCount(4, no benches) wrote a WARN (%q), want silence", warn.String())
			}
		})
	}
}
