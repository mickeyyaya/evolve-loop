package phasecontract

import (
	"os"
	"path/filepath"
	"testing"
)

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

// --- schema_version 2: optional failure block (ADR-0039 §7) ---

func TestParseVerdictSentinelFull_V2RoundTrip(t *testing.T) {
	f := &FailureBlock{
		Class:         "code-audit-fail",
		Defects:       []string{"d1: nil deref in walk()", "d2: missing clamp"},
		EvidencePaths: []string{"acs-verdict.json"},
	}
	line := RenderVerdictSentinelWithFailure("audit", "FAIL", f)
	s, ok := ParseVerdictSentinelFull("# Audit Report\n" + line + "\ntrailing\n")
	if !ok {
		t.Fatalf("v2 sentinel did not parse: %q", line)
	}
	if s.Phase != "audit" || s.Verdict != "FAIL" || s.SchemaVersion != SentinelSchemaVersionFailure {
		t.Errorf("header fields = %+v", s)
	}
	if s.Failure == nil || s.Failure.Class != "code-audit-fail" ||
		len(s.Failure.Defects) != 2 || len(s.Failure.EvidencePaths) != 1 {
		t.Errorf("failure block = %+v", s.Failure)
	}
}

// v1 compatibility is FOREVER: an absent failure block is legal (for PASS and
// for every artifact written before v2).
func TestParseVerdictSentinelFull_V1Compat(t *testing.T) {
	s, ok := ParseVerdictSentinelFull(RenderVerdictSentinel("build", "PASS"))
	if !ok || s.Verdict != "PASS" || s.Failure != nil {
		t.Fatalf("v1 sentinel must parse with nil failure; got (%+v,%v)", s, ok)
	}
}

func TestParseVerdictSentinelFull_MalformedTolerant(t *testing.T) {
	for _, c := range []string{"<!-- evolve-verdict: {not json} -->", "no sentinel", "<!-- evolve-verdict: {\"phase\":\"x\"} -->"} {
		if _, ok := ParseVerdictSentinelFull(c); ok {
			t.Errorf("malformed/verdict-less %q must yield ok=false", c)
		}
	}
}

// A nil failure renders byte-identical to the v1 line — producers without a
// failure to report keep the old shape (prompt/golden stability).
func TestRenderVerdictSentinelWithFailure_NilIsV1(t *testing.T) {
	if got, want := RenderVerdictSentinelWithFailure("audit", "PASS", nil), RenderVerdictSentinel("audit", "PASS"); got != want {
		t.Errorf("nil failure must be v1-identical:\n got %q\nwant %q", got, want)
	}
}

// The verdict-only wrapper and the full parser are the SAME parse (no dual
// parsers to drift).
func TestParseVerdictSentinel_DelegatesToFull(t *testing.T) {
	line := RenderVerdictSentinelWithFailure("tdd", "FAIL", &FailureBlock{Class: "code-build-fail"})
	v, ok := ParseVerdictSentinel(line)
	if !ok || v != "FAIL" {
		t.Fatalf("wrapper must parse v2 lines too; got (%q,%v)", v, ok)
	}
}

// ReadFailureBlock keeps scanning candidates: a registered artifact without a
// sentinel must not mask a conventional <phase>-report.md that carries the
// block (user phases may write both).
func TestReadFailureBlock_FallsThroughCandidates(t *testing.T) {
	ws := t.TempDir()
	// tdd's registered artifact is test-report.md — write it sentinel-less.
	mustWrite(t, ws, "test-report.md", "## Tests\nprose only\n")
	mustWrite(t, ws, "tdd-report.md", "## Tests\n"+
		RenderVerdictSentinelWithFailure("tdd", "FAIL", &FailureBlock{Class: "code-build-fail"})+"\n")
	fb, ok := ReadFailureBlock(ws, "tdd")
	if !ok || fb.Class != "code-build-fail" {
		t.Fatalf("got (%+v,%v), want the conventional candidate's block", fb, ok)
	}
}

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
