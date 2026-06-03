package phasecontract

import "testing"

// Layer 5 (ADR-0034): a machine-readable verdict sentinel removes the verdict-
// drift class. Classifiers read the sentinel FIRST, then fall back to the legacy
// regex-on-prose (strangler fig — the old path stays as fallback).

func TestParseVerdictSentinel_Basic(t *testing.T) {
	content := "# Audit Report\n\nblah\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->\nmore\n"
	v, ok := ParseVerdictSentinel(content)
	if !ok || v != "PASS" {
		t.Fatalf("got (%q,%v), want (PASS,true)", v, ok)
	}
}

func TestParseVerdictSentinel_None(t *testing.T) {
	if v, ok := ParseVerdictSentinel("no sentinel here\nVerdict: PASS\n"); ok {
		t.Fatalf("got (%q,true), want (_,false) — prose is not a sentinel", v)
	}
}

func TestParseVerdictSentinel_Malformed_FallsThrough(t *testing.T) {
	// Tolerant: a malformed sentinel must NOT be treated as a verdict; the caller
	// falls back to the regex path.
	if _, ok := ParseVerdictSentinel("<!-- evolve-verdict: {not json} -->"); ok {
		t.Fatal("malformed sentinel must yield ok=false so the regex fallback runs")
	}
}

func TestRenderVerdictSentinel_RoundTrips(t *testing.T) {
	line := RenderVerdictSentinel("build", "PASS")
	v, ok := ParseVerdictSentinel("## Changes\n- x\n" + line + "\n")
	if !ok || v != "PASS" {
		t.Fatalf("round-trip got (%q,%v), want (PASS,true); line=%q", v, ok, line)
	}
}
