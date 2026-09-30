//go:build acs

package cycle1299

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func fixturePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phasecontract", "testdata", "cycle1298-quoted-decoys.md")
}

func readFixture(t *testing.T) string {
	t.Helper()
	path := fixturePath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cycle-1298 regression fixture missing at %s: %v", path, err)
	}
	return string(raw)
}

func TestC1299_001_UnparseableEarlierDecoyLosesToTail(t *testing.T) {
	doc := strings.Join([]string{
		"# Adversarial review",
		"The gate reads `<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"WARN\",…} -->` first.",
		"",
		phasecontract.RenderVerdictSentinel("audit", "FAIL"),
	}, "\n")

	s, ok := phasecontract.ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull: ok=false — an unparseable quoted decoy blanked the real tail sentinel")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL (the tail sentinel must win)", s.Verdict)
	}
}

func TestC1299_002_WellFormedEarlierDecoyLosesToTail(t *testing.T) {
	doc := strings.Join([]string{
		"Contract example quoted in prose:",
		phasecontract.RenderVerdictSentinel("audit", "PASS"),
		"The phase's actual verdict:",
		phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", &phasecontract.FailureBlock{
			Class:   "gate_bypass",
			Defects: []string{"F-1 HIGH: first-match sentinel selection"},
		}),
	}, "\n")

	s, ok := phasecontract.ParseVerdictSentinelFull(doc)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull: ok=false, want ok=true")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL — an earlier well-formed decoy won over the tail sentinel", s.Verdict)
	}
	if s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("failure = %+v, want class=gate_bypass carried from the tail sentinel", s.Failure)
	}
}

func TestC1299_003_AllMalformedStillDeclines(t *testing.T) {
	cases := map[string]string{
		"elided-json":   "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":…} -->",
		"not-json":      "<!-- evolve-verdict: {not json at all} -->",
		"verdict-empty": "<!-- evolve-verdict: {\"phase\":\"audit\",\"schema_version\":1} -->",
		"all-three": strings.Join([]string{
			"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":…} -->",
			"<!-- evolve-verdict: {not json at all} -->",
			"<!-- evolve-verdict: {\"phase\":\"audit\",\"schema_version\":1} -->",
		}, "\n"),
		"lone-placeholder-echo": phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
			&phasecontract.FailureBlock{Class: "<failure class>", Defects: []string{"<one line per defect>"}}),
	}
	for name, doc := range cases {
		if s, ok := phasecontract.ParseVerdictSentinelFull(doc); ok {
			t.Errorf("%s: ParseVerdictSentinelFull = (%+v, true), want ok=false", name, s)
		}
	}
}

func TestC1299_004_NoSentinelUnchanged(t *testing.T) {
	for _, doc := range []string{"", "# Report\n\nno sentinel here\n", "<!-- evolve-verdict: -->", "<!-- evolve-verdict: {} -->"} {
		if s, ok := phasecontract.ParseVerdictSentinelFull(doc); ok {
			t.Errorf("ParseVerdictSentinelFull(%q) = (%+v, true), want ok=false", doc, s)
		}
	}
}

func TestC1299_005_LiveCycle1298FixtureParsesFail(t *testing.T) {
	content := readFixture(t)

	if n := strings.Count(content, "evolve-verdict:"); n < 3 {
		t.Fatalf("fixture holds %d evolve-verdict occurrences, want the multi-decoy cycle-1298 shape (>=3)", n)
	}

	s, ok := phasecontract.ParseVerdictSentinelFull(content)
	if !ok {
		t.Fatalf("ParseVerdictSentinelFull(cycle-1298 report): ok=false — quoted decoys still blank the real tail sentinel")
	}
	if s.Verdict != "FAIL" {
		t.Errorf("verdict = %q, want FAIL", s.Verdict)
	}
	if s.Failure == nil || s.Failure.Class != "gate_bypass" {
		t.Errorf("failure = %+v, want class=gate_bypass", s.Failure)
	}
}

func TestC1299_006_FirstMatchSelectionWouldBeWrong(t *testing.T) {
	content := readFixture(t)

	legacyRE := regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)
	legacyVerdict, legacyOK := "", false
	if m := legacyRE.FindStringSubmatch(content); m != nil {
		var legacy phasecontract.VerdictSentinel
		if err := json.Unmarshal([]byte(m[1]), &legacy); err == nil && legacy.Verdict != "" {
			legacyVerdict, legacyOK = legacy.Verdict, true
		}
	}
	if legacyOK && legacyVerdict == "FAIL" {
		t.Fatalf("fixture no longer discriminates: first-match selection already yields FAIL")
	}

	s, ok := phasecontract.ParseVerdictSentinelFull(content)
	if !ok || s.Verdict != "FAIL" {
		t.Errorf("production parser = (%q, %v), want (FAIL, true); legacy first-match = (%q, %v)",
			s.Verdict, ok, legacyVerdict, legacyOK)
	}
}

func TestC1299_007_ReadFailureBlockReachesTailSentinel(t *testing.T) {
	content := readFixture(t)

	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "adversarial-review-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write phase report: %v", err)
	}

	fb, ok := phasecontract.ReadFailureBlock(ws, "adversarial-review")
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
