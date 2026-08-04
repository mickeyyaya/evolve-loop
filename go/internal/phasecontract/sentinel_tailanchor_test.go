package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cycle-1299 RED contract — tail-anchored verdict-sentinel selection.
//
// ParseVerdictSentinelFull used FindStringSubmatch, which returns the FIRST
// structural match anywhere in the document. A report whose PROSE quotes
// example `<!-- evolve-verdict: {...} -->` syntax therefore beat the real
// sentinel at the tail: a quoted decoy that merely unmarshals silently won,
// and a quoted decoy with elided/unparseable JSON blanked the whole read
// (unmarshal failure returns ok=false rather than trying the next candidate).
// Both shapes are live in the cycle-1298 adversarial-review report, which fired
// [bad_verdict] x3 and circuit-opened the contract gate enforce→advisory.
//
// The contract these tests pin: walk candidates from the END and return the
// LAST one that unmarshals cleanly, carries a non-empty Verdict, and is not a
// placeholder echo. Single-sentinel documents (the overwhelmingly common case)
// are unaffected — a one-element candidate list behaves exactly as before.

const tailAnchorFixture = "testdata/cycle1298-quoted-decoys.md"

// AC1 — a malformed/elided decoy earlier in the document must not blank or win
// over a well-formed sentinel at the tail.
func TestSentinelTailAnchor_MalformedEarlierDecoyLosesToTail(t *testing.T) {
	doc := strings.Join([]string{
		"# Report",
		"Prose quoting the shape: `<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\",…} -->`",
		"More prose.",
		RenderVerdictSentinel("audit", "FAIL"),
	}, "\n")

	s, ok := ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull: ok=false — an unparseable earlier decoy blanked the real tail sentinel")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want %q (tail sentinel must win)", s.Verdict, "FAIL")
	}
}

// AC1 (semantic sibling) — a decoy that DOES unmarshal cleanly still must not
// win over the tail sentinel. This separates a real tail-anchor fix from the
// partial fix "keep scanning until something unmarshals", which would return
// the earlier PASS decoy here.
func TestSentinelTailAnchor_ValidEarlierDecoyLosesToTail(t *testing.T) {
	doc := strings.Join([]string{
		"Contract example, quoted in prose:",
		RenderVerdictSentinel("audit", "PASS"),
		"...and the phase's actual verdict:",
		RenderVerdictSentinelWithFailure("audit", "FAIL", &FailureBlock{Class: "gate_bypass"}),
	}, "\n")

	s, ok := ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull: ok=false, want ok=true")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want %q — an earlier well-formed decoy won over the tail sentinel", s.Verdict, "FAIL")
	}
	if s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("failure block = %+v, want class=gate_bypass from the tail sentinel", s.Failure)
	}
}

// AC2 (negative) — when every sentinel-shaped substring is garbage, the tolerant
// reader still declines, so the caller's legacy parser runs. Tail-anchoring must
// not manufacture a verdict out of malformed candidates.
func TestSentinelTailAnchor_AllMalformedStillNotOK(t *testing.T) {
	doc := strings.Join([]string{
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":…} -->",
		"<!-- evolve-verdict: {not json at all} -->",
		"<!-- evolve-verdict: {\"phase\":\"audit\"} -->", // parses, but verdict is empty
	}, "\n")

	if s, ok := ParseVerdictSentinelFull(doc); ok {
		t.Errorf("ParseVerdictSentinelFull = (%+v, true), want ok=false — no candidate is well-formed", s)
	}
}

// AC3 (edge) — zero sentinel comments is unchanged behaviour: ok=false.
func TestSentinelTailAnchor_NoSentinelStillNotOK(t *testing.T) {
	for _, doc := range []string{"", "# Report\n\nno sentinel here\n", "<!-- evolve-verdict: -->"} {
		if s, ok := ParseVerdictSentinelFull(doc); ok {
			t.Errorf("ParseVerdictSentinelFull(%q) = (%+v, true), want ok=false", doc, s)
		}
	}
}

// Cycle-603 preservation — a document whose ONLY sentinel is a contract-example
// placeholder echo must still decline. Tail-anchoring must not weaken this.
func TestSentinelTailAnchor_LonePlaceholderEchoStillNotOK(t *testing.T) {
	doc := "prose\n" + RenderVerdictSentinelWithFailure("audit", "FAIL", &FailureBlock{
		Class:   "<failure class>",
		Defects: []string{"<one line per defect>"},
	})

	if s, ok := ParseVerdictSentinelFull(doc); ok {
		t.Errorf("ParseVerdictSentinelFull = (%+v, true), want ok=false for a lone placeholder echo", s)
	}
}

// Tail placeholder echo + a real sentinel earlier: the walk skips invalid
// candidates and keeps going backwards to the last VALID one (scout H2 —
// "last parseable", not "last raw match").
func TestSentinelTailAnchor_SkipsInvalidTailToLastValid(t *testing.T) {
	doc := strings.Join([]string{
		"The real verdict:",
		RenderVerdictSentinelWithFailure("audit", "FAIL", &FailureBlock{Class: "gate_bypass"}),
		"The contract's own printed example, echoed from scrollback:",
		RenderVerdictSentinelWithFailure("audit", "PASS", &FailureBlock{
			Class:   "<failure class>",
			Defects: []string{"<one line per defect>"},
		}),
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":…} -->",
	}, "\n")

	s, ok := ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull: ok=false — invalid tail candidates must be skipped, not fatal")
	}
	if s.Verdict != "FAIL" || s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("got verdict=%q failure=%+v, want the last VALID candidate (FAIL/gate_bypass)", s.Verdict, s.Failure)
	}
}

// AC4 — the LIVE cycle-1298 report (5 quoted decoys in prose + the real
// sentinel at the tail), parsed through the same function every gate caller
// uses. This is the wiring proof against a real captured artifact, not a
// synthetic string.
func TestSentinelTailAnchor_LiveCycle1298Fixture(t *testing.T) {
	raw, err := os.ReadFile(tailAnchorFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	s, ok := ParseVerdictSentinelFull(string(raw))
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull(cycle-1298 report): ok=false — the quoted decoys blanked the real tail sentinel")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL", s.Verdict)
	}
	if s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("failure = %+v, want class=gate_bypass", s.Failure)
	}
}

// AC5 — anti-vacuity. Reconstruct the OLD first-match selection over the same
// regex and the same fixture, and assert it produces a DIFFERENT (wrong)
// answer. If this ever fails, the fixture stopped discriminating fixed from
// broken and the AC4 assertion above would be vacuously true.
func TestSentinelTailAnchor_FirstMatchSelectionIsGone(t *testing.T) {
	raw, err := os.ReadFile(tailAnchorFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	content := string(raw)

	// The legacy selection, verbatim: first structural match, no fallthrough.
	legacyVerdict, legacyOK := "", false
	if m := sentinelRE.FindStringSubmatch(content); m != nil {
		var legacy VerdictSentinel
		if err := json.Unmarshal([]byte(m[1]), &legacy); err == nil && legacy.Verdict != "" {
			legacyVerdict, legacyOK = legacy.Verdict, true
		}
	}
	if legacyOK && legacyVerdict == "FAIL" {
		t.Fatalf("fixture no longer discriminates: first-match selection already yields FAIL")
	}

	s, ok := ParseVerdictSentinelFull(content)
	if !ok || s.Verdict != "FAIL" {
		t.Errorf("current parser = (%q, %v), want (FAIL, true) while legacy first-match = (%q, %v)",
			s.Verdict, ok, legacyVerdict, legacyOK)
	}
}

// Wiring proof — drive the production caller. ReadFailureBlock is the ONE
// reader router signal-lifting, orchestrator faillearn and the classifier go
// through; it must surface the tail sentinel's failure block off the live
// fixture laid down as a phase report.
func TestSentinelTailAnchor_ReadFailureBlockReachesTailSentinel(t *testing.T) {
	raw, err := os.ReadFile(tailAnchorFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "adversarial-review-report.md"), raw, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	fb, ok := ReadFailureBlock(ws, "adversarial-review")
	if !ok {
		t.Fatalf("ReadFailureBlock: ok=false — the production reader still misses the tail sentinel")
	}
	if fb.Class != "gate_bypass" {
		t.Errorf("failure class = %q, want gate_bypass", fb.Class)
	}
	if len(fb.Defects) == 0 {
		t.Errorf("defects empty, want the tail sentinel's defect list")
	}
}
