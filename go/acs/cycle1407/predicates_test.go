//go:build acs

package cycle1407

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const decoyFixtureRel = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"

const regressionTestRel = "go/internal/deliverable/salvage_instrument_test.go"

func readDecoyFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(acsassert.RepoRoot(t), decoyFixtureRel)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read the cycle-1298 quoted-decoy corpus at %s: %v\n"+
			"This suite is defined against that exact file; if it moved, re-point decoyFixtureRel "+
			"rather than copying its bytes (single-source-of-truth).", p, err)
	}
	return string(raw)
}

func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code
}

func goPkg(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", rel)
}

func baselineLine(t *testing.T, eventType, pattern string, recoverable bool) string {
	t.Helper()
	rec := map[string]any{
		"event_type":  eventType,
		"timestamp":   "2026-08-10T00:00:00Z",
		"severity":    "info",
		"phase":       "audit",
		"recoverable": recoverable,
		"pattern":     pattern,
		"reason":      "seeded by cycle-1407 predicate",
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal seeded baseline record: %v", err)
	}
	return string(b)
}

func seedBaseline(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	lines := []string{
		baselineLine(t, "bad_verdict_classified", "trailing-comma", true),
		baselineLine(t, "bad_verdict_classified", "trailing-comma", true),
		baselineLine(t, "bad_verdict_classified", "fenced-json", true),
		baselineLine(t, "bad_verdict_classified", "", false),
		baselineLine(t, "bad_verdict_classified", "", false),
		baselineLine(t, "phase_started", "", false),
	}
	p := filepath.Join(root, ".evolve", "bad-verdict-baseline.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write seeded baseline: %v", err)
	}
	return root
}

func TestC1407_001_summarizer_computes_rate_and_pattern_counts(t *testing.T) {
	root := seedBaseline(t)
	f, err := os.Open(filepath.Join(root, ".evolve", "bad-verdict-baseline.jsonl"))
	if err != nil {
		t.Fatalf("open seeded baseline: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := deliverable.SummarizeBadVerdictBaseline(f)
	if err != nil {
		t.Fatalf("SummarizeBadVerdictBaseline over a well-formed baseline returned error: %v", err)
	}

	if got.Total != 5 {
		t.Errorf("Total = %d, want 5 (6 lines seeded, but phase_started is a foreign event and must not be counted)", got.Total)
	}
	if got.Recoverable != 3 {
		t.Errorf("Recoverable = %d, want 3", got.Recoverable)
	}
	if got.Rate != 0.6 {
		t.Errorf("Rate = %v, want 0.6 (3 recoverable / 5 classified)", got.Rate)
	}
	wantByPattern := map[deliverable.SalvagePattern]int{
		deliverable.SalvagePatternTrailingComma: 2,
		deliverable.SalvagePatternFencedJSON:    1,
	}
	for pat, want := range wantByPattern {
		if got.ByPattern[pat] != want {
			t.Errorf("ByPattern[%q] = %d, want %d", pat, got.ByPattern[pat], want)
		}
	}
	if n, ok := got.ByPattern[deliverable.SalvagePatternDisplaced]; ok && n != 0 {
		t.Errorf("ByPattern[%q] = %d, want absent-or-zero: no displaced-line record was seeded",
			deliverable.SalvagePatternDisplaced, n)
	}
}

func TestC1407_002_summarizer_rejects_malformed_and_survives_empty(t *testing.T) {
	empty, err := deliverable.SummarizeBadVerdictBaseline(strings.NewReader(""))
	if err != nil {
		t.Fatalf("empty baseline returned error %v: an un-populated sidecar is the normal fresh-root state, not a failure", err)
	}
	if empty.Total != 0 {
		t.Errorf("empty baseline Total = %d, want 0", empty.Total)
	}
	if empty.Rate != 0 {
		t.Errorf("empty baseline Rate = %v, want 0 (a 0/0 division must not yield NaN)", empty.Rate)
	}
	if empty.Rate != empty.Rate {
		t.Errorf("empty baseline Rate is NaN — 0/0 was evaluated unguarded")
	}

	torn := baselineLine(t, "bad_verdict_classified", "fenced-json", true) + "\n{\"event_type\":\"bad_verd"
	if _, err := deliverable.SummarizeBadVerdictBaseline(strings.NewReader(torn)); err == nil {
		t.Errorf("a truncated JSONL line was accepted silently: want a loud error, because dropping unparseable " +
			"lines under-counts the denominator and biases the recoverable-malformed rate")
	}
}

func TestC1407_003_evolve_salvage_report_surfaces_rate_from_real_cli(t *testing.T) {
	root := seedBaseline(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)

	stdout, stderr, code := acsSubprocess(t, "go", "run", goPkg(t, "cmd/evolve"), "salvage", "report", "-json")
	if code != 0 {
		t.Fatalf("`evolve salvage report -json` exited %d: the summarizer has no production caller yet\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}

	var out struct {
		Total       int            `json:"total"`
		Recoverable int            `json:"recoverable"`
		Rate        float64        `json:"rate"`
		ByPattern   map[string]int `json:"by_pattern"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("`evolve salvage report -json` stdout is not the documented JSON envelope: %v\nstdout:\n%s", err, stdout)
	}
	if out.Total != 5 || out.Recoverable != 3 || out.Rate != 0.6 {
		t.Errorf("CLI reported total=%d recoverable=%d rate=%v; want 5/3/0.6 — the CLI is not reading the real baseline",
			out.Total, out.Recoverable, out.Rate)
	}
	if out.ByPattern["trailing-comma"] != 2 || out.ByPattern["fenced-json"] != 1 {
		t.Errorf("CLI by_pattern = %v; want trailing-comma=2 fenced-json=1 (the breakdown, not just the headline rate)", out.ByPattern)
	}
}

func TestC1407_004_readme_documents_a_command_that_actually_runs(t *testing.T) {
	readme := filepath.Join(acsassert.RepoRoot(t), "docs/research/deliverable-alignment-2026-08/README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read %s: %v", readme, err)
	}
	body := string(raw)

	const invocation = "evolve salvage report"
	if !strings.Contains(body, invocation) {
		t.Fatalf("README does not document the %q surface: the extraction gate's own rate is undiscoverable "+
			"to an operator who has not read the Go source", invocation)
	}
	for _, word := range []string{"rate", "pattern"} {
		if !strings.Contains(strings.ToLower(body), word) {
			t.Errorf("README never mentions %q — the issue/gap/solution paragraph must say what the number IS, "+
				"not merely that a command exists", word)
		}
	}

	root := seedBaseline(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	stdout, stderr, code := acsSubprocess(t, "go", "run", goPkg(t, "cmd/evolve"), "salvage", "report")
	if code != 0 {
		t.Fatalf("the README-documented invocation %q exited %d — documented but not runnable\nstdout:\n%s\nstderr:\n%s",
			invocation, code, stdout, stderr)
	}
	if !strings.Contains(stdout, "0.6") && !strings.Contains(stdout, "60") {
		t.Errorf("human-readable `%s` never prints the 0.6 (60%%) rate it exists to surface; got:\n%s", invocation, stdout)
	}
}

func TestC1407_005_quoted_decoy_corpus_alone_is_not_recoverable(t *testing.T) {
	got := deliverable.ClassifyBadVerdict(readDecoyFixture(t))
	if got.Recoverable {
		t.Errorf("the cycle-1298 corpus classified Recoverable=true (pattern=%q, reason=%q): its sentinel-shaped "+
			"spans are prose echoes of OTHER phases' sentinels, and treating an echo as a salvage signal is the "+
			"cycle-641 lesson verbatim", got.Pattern, got.Reason)
	}
	if got.Reason == "" {
		t.Errorf("classification carries an empty Reason: a silent classification is not observability")
	}
}

func TestC1407_006_real_tail_sentinel_classifies_through_quoted_decoys(t *testing.T) {
	const malformedTail = "\n\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":2,} -->\n"
	got := deliverable.ClassifyBadVerdict(readDecoyFixture(t) + malformedTail)

	if !got.Recoverable {
		t.Errorf("Recoverable=false (reason=%q): the report's own tail sentinel has a trailing comma before its "+
			"closing brace and is plainly recoverable, but the classifier stopped at a quoted decoy earlier in the "+
			"prose. A classifier must not key off a span that is a verbatim echo of another phase's sentinel.", got.Reason)
	}
	if got.Pattern != deliverable.SalvagePatternTrailingComma {
		t.Errorf("Pattern = %q, want %q — classified from the wrong span",
			got.Pattern, deliverable.SalvagePatternTrailingComma)
	}
}

func TestC1407_007_decoy_quoted_after_the_real_sentinel_is_ignored(t *testing.T) {
	const quotedDecoyTail = "\n\nFor example, an auditor might paste " +
		"`<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":1,} -->` " +
		"into prose while explaining the bypass; that is illustration, not this report's verdict.\n"
	got := deliverable.ClassifyBadVerdict(readDecoyFixture(t) + quotedDecoyTail)

	if got.Recoverable {
		t.Errorf("Recoverable=true (pattern=%q): the only malformed sentinel in this document is explicitly quoted "+
			"inside backticks as an illustration, and the report's own sentinel parsed cleanly. Selecting the LAST "+
			"sentinel is not decoy immunity — the fix must exclude quoted/echoed spans.", got.Pattern)
	}
}

func TestC1407_008_regression_case_lands_in_the_package_suite(t *testing.T) {
	src := filepath.Join(acsassert.RepoRoot(t), regressionTestRel)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if !strings.Contains(string(raw), "cycle1298-quoted-decoys.md") {
		t.Errorf("%s does not read the cycle-1298 corpus by path: the regression case must reference the one "+
			"canonical fixture, not a re-typed excerpt of it", regressionTestRel)
	}

	stdout, stderr, code := acsSubprocess(t, "go", "test", "-count=1", "-v",
		"-run", "TestClassifyBadVerdict", goPkg(t, "internal/deliverable"))
	combined := stdout + stderr
	if code != 0 {
		t.Fatalf("`go test -run TestClassifyBadVerdict ./internal/deliverable` exited %d:\n%s", code, combined)
	}
	if !strings.Contains(combined, "--- PASS:") {
		t.Fatalf("no `--- PASS:` line — the -run pattern matched no test at all (exit 0 proves nothing here):\n%s", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "decoy") {
		t.Errorf("no executed subtest names a decoy case; Task B's regression subtest is absent from the package "+
			"suite, so the property survives only in this cycle-scoped ACS file:\n%s", combined)
	}
}

func TestC1407_009_new_exported_symbols_pass_the_apicover_gate(t *testing.T) {
	named := filepath.Join(acsassert.RepoRoot(t), "go/internal/deliverable/apicover_named_test.go")
	raw, err := os.ReadFile(named)
	if err != nil {
		t.Fatalf("read %s: %v", named, err)
	}
	for _, sym := range []string{"SummarizeBadVerdictBaseline", "BaselineSummary"} {
		if !strings.Contains(string(raw), sym) {
			t.Errorf("apicover_named_test.go never names %s: ./internal/deliverable is enrolled in "+
				".apicover-enforce, so a new exported symbol that is not named there fails the repo-wide gate", sym)
		}
	}

	tmp := t.TempDir()
	pkg := goPkg(t, "internal/deliverable")
	profile := filepath.Join(tmp, "cover.out")
	if _, stderr, code := acsSubprocess(t, "go", "test", "-count=1",
		"-coverprofile="+profile, pkg); code != 0 {
		t.Fatalf("coverage run over internal/deliverable exited %d:\n%s", code, stderr)
	}
	funcTxt, _, code := acsSubprocess(t, "go", "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover -func exited %d", code)
	}
	funcPath := filepath.Join(tmp, "coverage.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcTxt), 0o644); err != nil {
		t.Fatalf("write coverage.func.txt: %v", err)
	}

	stdout, stderr, code := acsSubprocess(t, "go", "run", goPkg(t, "cmd/apicover"),
		"-enforce", "-cover", funcPath, pkg)
	if code != 0 {
		t.Errorf("`apicover -enforce -cover` on internal/deliverable exited %d — a newly exported symbol is "+
			"uncovered or false-green, which hard-fails the repo-wide ADR-0069 gate at build time\nstdout:\n%s\nstderr:\n%s",
			code, stdout, stderr)
	}
}
