//go:build acs

// Package cycle1299 materialises the cycle-1299 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `sentinel-parse-tail-anchor`):
//
//   - sentinel-tail-anchor-parse        → tail-anchored verdict-sentinel selection
//   - sentinel-tail-anchor-live-fixture → the cycle-1298 report as regression proof
//
// The defect. `ParseVerdictSentinelFull` selected with `FindStringSubmatch`, which
// returns the FIRST structural match in the document. A report that QUOTES the
// sentinel shape in prose therefore beat the real sentinel at the tail: a quoted
// decoy that unmarshals silently won, and a quoted decoy with elided JSON blanked
// the whole read (unmarshal failure returns ok=false instead of trying the next
// candidate). Both shapes are live in `.evolve/runs/cycle-1298/adversarial-review-report.md`
// — 5 quoted decoys + 1 real tail sentinel — which fired [bad_verdict] x3 and
// circuit-opened the contract gate enforce→advisory.
//
// Predicate strategy — every predicate below CALLS the production parser
// (`phasecontract.ParseVerdictSentinelFull`) or the production reader
// (`phasecontract.ReadFailureBlock`) and asserts on its return value. None of
// them greps sentinel.go for a magic string, so adding `FindAllStringSubmatch`
// to the source without correct tail-walk semantics does not green them (the
// cycle-85 degenerate-predicate ban).
//
//   - 001 AC1: an earlier UNPARSEABLE decoy must not blank the real tail sentinel.
//   - 002 AC1': an earlier WELL-FORMED decoy must not win either — this is what
//     separates a real tail anchor from the partial fix "scan until something
//     unmarshals".
//   - 003 AC2 (negative): all-malformed candidates still yield ok=false, so the
//     caller's legacy parser runs. Anti-gaming: a parser that returns the last
//     RAW match regardless of validity fails here.
//   - 004 AC3 (edge): zero sentinels, empty input, and an empty payload are
//     unchanged (ok=false).
//   - 005 AC4: the LIVE cycle-1298 fixture, read from testdata, parses to
//     verdict=FAIL / class=gate_bypass through the real gate-caller function.
//   - 006 AC5 (anti-vacuity): the legacy first-match selection, reconstructed in
//     the predicate over the same fixture, produces a DIFFERENT (wrong) answer —
//     proof the fixture discriminates fixed from broken.
//   - 007 wiring proof: `ReadFailureBlock`, the single production reader the
//     router / faillearn / classifier go through, surfaces the tail sentinel's
//     failure block off the fixture laid down as a phase report.
//
// Roots: the fixture is a Builder/TDD deliverable landing in the worktree, so it
// is resolved under acsassert.RepoRoot (the SOURCE root). Its absence is a
// FAILURE, not a skip.
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

// fixturePath resolves the committed cycle-1298 regression fixture.
func fixturePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", "internal", "phasecontract", "testdata", "cycle1298-quoted-decoys.md")
}

// readFixture returns the fixture contents, failing (never skipping) when the
// deliverable is absent.
func readFixture(t *testing.T) string {
	t.Helper()
	path := fixturePath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cycle-1298 regression fixture missing at %s: %v", path, err)
	}
	return string(raw)
}

// TestC1299_001_UnparseableEarlierDecoyLosesToTail pins AC1: a prose-quoted
// sentinel with elided JSON must not blank the well-formed tail sentinel.
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

// TestC1299_002_WellFormedEarlierDecoyLosesToTail pins AC1': tail-anchored, not
// merely "first candidate that happens to unmarshal".
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

// TestC1299_003_AllMalformedStillDeclines pins AC2 (negative axis): the tolerant
// reader must not manufacture a verdict from garbage candidates.
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

// TestC1299_004_NoSentinelUnchanged pins AC3 (edge/OOD axis).
func TestC1299_004_NoSentinelUnchanged(t *testing.T) {
	for _, doc := range []string{"", "# Report\n\nno sentinel here\n", "<!-- evolve-verdict: -->", "<!-- evolve-verdict: {} -->"} {
		if s, ok := phasecontract.ParseVerdictSentinelFull(doc); ok {
			t.Errorf("ParseVerdictSentinelFull(%q) = (%+v, true), want ok=false", doc, s)
		}
	}
}

// TestC1299_005_LiveCycle1298FixtureParsesFail pins AC4: the real captured
// report (5 quoted decoys + 1 real tail sentinel) through the LIVE parser.
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

// TestC1299_006_FirstMatchSelectionWouldBeWrong pins AC5 (anti-vacuity): the
// legacy selection, reconstructed here, must disagree with the fixed parser on
// this fixture. If it ever agrees, 005 has gone vacuously true.
func TestC1299_006_FirstMatchSelectionWouldBeWrong(t *testing.T) {
	content := readFixture(t)

	// The legacy semantics, reconstructed: first structural match, no fallthrough.
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

// TestC1299_007_ReadFailureBlockReachesTailSentinel is the wiring proof: it
// drives ReadFailureBlock — the ONE production reader router signal-lifting,
// orchestrator faillearn and the classifier all go through — over the live
// fixture laid down as a phase report on disk.
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
