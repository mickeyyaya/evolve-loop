package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tailAnchorFixture = "testdata/cycle1298-quoted-decoys.md"

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

func TestSentinelTailAnchor_NoSentinelStillNotOK(t *testing.T) {
	for _, doc := range []string{"", "# Report\n\nno sentinel here\n", "<!-- evolve-verdict: -->"} {
		if s, ok := ParseVerdictSentinelFull(doc); ok {
			t.Errorf("ParseVerdictSentinelFull(%q) = (%+v, true), want ok=false", doc, s)
		}
	}
}

func TestSentinelTailAnchor_LonePlaceholderEchoStillNotOK(t *testing.T) {
	doc := "prose\n" + RenderVerdictSentinelWithFailure("audit", "FAIL", &FailureBlock{
		Class:   "<failure class>",
		Defects: []string{"<one line per defect>"},
	})

	if s, ok := ParseVerdictSentinelFull(doc); ok {
		t.Errorf("ParseVerdictSentinelFull = (%+v, true), want ok=false for a lone placeholder echo", s)
	}
}

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

func TestSentinelTailAnchor_FirstMatchSelectionIsGone(t *testing.T) {
	raw, err := os.ReadFile(tailAnchorFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	content := string(raw)

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
