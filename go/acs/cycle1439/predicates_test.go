//go:build acs

package cycle1439

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const malformedTailSentinel = "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,} -->\n"

const cleanTailSentinel = "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":2} -->\n"

const quotedRecoverableDecoy = "The contract shape is " +
	"`<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":1,} -->` " +
	"— note the stray comma an agent often leaves behind.\n"

const quotedUnrecoverableDecoy = "Another phase emitted " +
	"`<!-- evolve-verdict: {\"phase\":\"audit\" \"verdict\":\"FAIL\"} -->` " +
	"which the strict parser rejected outright.\n"

func TestC1439_001_UnmatchedBacktickDoesNotSuppressOwnSentinel(t *testing.T) {
	t.Parallel()
	const content = "## Verdict\n" +
		"An unrelated inline code span ends here`" + malformedTailSentinel +
		"No other verdict object appears anywhere in this report.\n"

	got := deliverable.ClassifyBadVerdict(content)
	if !got.Recoverable || got.Pattern != deliverable.SalvagePatternTrailingComma {
		t.Fatalf("F1: one unmatched (never-closing) backtick before the report's OWN tail sentinel suppressed it "+
			"as a quoted echo — got Recoverable=%v Pattern=%q Reason=%q, want Recoverable=true Pattern=%q. "+
			"isQuotedEcho must require the adjacent backtick run to actually CLOSE, not trust single-character "+
			"adjacency (go/internal/deliverable/salvage_instrument.go).",
			got.Recoverable, got.Pattern, got.Reason, deliverable.SalvagePatternTrailingComma)
	}
	if got.Reason == "" {
		t.Error("Reason is empty — a silent classification is not observability")
	}
}

func TestC1439_002_QuotedDecoyIsNotTheReportsOwnVerdict(t *testing.T) {
	t.Parallel()
	content := "# Audit Report\n\n" + quotedRecoverableDecoy + "\n## Verdict\n" + cleanTailSentinel

	got := deliverable.ClassifyBadVerdict(content)
	if got.Recoverable {
		t.Errorf("Recoverable=true (Pattern=%q, Reason=%q): the only malformed sentinel is explicitly wrapped in "+
			"balanced backticks as prose illustration, and this report's OWN sentinel parses cleanly. A quoted echo "+
			"must be excised before classification (cycle-641: classifiers MUST exclude verbatim echoes of injected "+
			"contract text).", got.Pattern, got.Reason)
	}
	if got.Reason == "" {
		t.Error("Reason is empty — a silent classification is not observability")
	}
}

func TestC1439_003_RealTailSentinelClassifiesThroughQuotedDecoy(t *testing.T) {
	t.Parallel()
	content := "# Audit Report\n\n" + quotedUnrecoverableDecoy + "\n## Verdict\n" + malformedTailSentinel

	got := deliverable.ClassifyBadVerdict(content)
	if !got.Recoverable || got.Pattern != deliverable.SalvagePatternTrailingComma {
		t.Errorf("got Recoverable=%v Pattern=%q Reason=%q, want Recoverable=true Pattern=%q — the classifier stopped "+
			"at a backticked echo of another phase's verdict instead of reaching this report's own trailing-comma "+
			"tail sentinel.", got.Recoverable, got.Pattern, got.Reason, deliverable.SalvagePatternTrailingComma)
	}
}

func TestC1439_004_QuotedDecoyAfterRealSentinelIgnored(t *testing.T) {
	t.Parallel()
	content := "# Audit Report\n\n## Verdict\n" + cleanTailSentinel +
		"\nFor example, an agent might paste " + quotedRecoverableDecoy

	got := deliverable.ClassifyBadVerdict(content)
	if got.Recoverable {
		t.Errorf("Recoverable=true (Pattern=%q): selecting the LAST sentinel is not decoy immunity — the trailing "+
			"span is backticked illustration and this report's own verdict, above it, parsed cleanly.", got.Pattern)
	}
}

func TestC1439_005_BacktickAtContentBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		content string
	}{
		{"sentinel at offset zero", malformedTailSentinel + "trailing prose with no backticks\n"},
		{"sentinel flush at end", "leading prose\n" + strings.TrimSuffix(malformedTailSentinel, "\n")},
		{"backtick flush at end", "leading prose\n" + strings.TrimSuffix(malformedTailSentinel, "\n") + "`"},
		{"lone backtick only", "`"},
		{"empty document", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ClassifyBadVerdict panicked on %q: %v — a boundary-unguarded backtick peek "+
						"(content[start-1] / content[end]) crashes the caller phase", tc.name, r)
				}
			}()
			got := deliverable.ClassifyBadVerdict(tc.content)
			if got.Reason == "" {
				t.Errorf("%s: Reason is empty — every classification must say why", tc.name)
			}
		})
	}
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	bin := filepath.Join(t.TempDir(), "evolve")
	_, stderr, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "build", "-o", bin, "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("go build ./cmd/evolve: exit=%d err=%v\n%s", code, err, stderr)
	}
	return bin
}

func writeBaselineProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join([]string{
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"trailing-comma"}`,
		`{"event_type":"some_other_emitter","recoverable":true,"pattern":"fenced-json"}`,
		``,
		`{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"fenced-json"}`,
		`{"event_type":"bad_verdict_classified","recoverable":false,"pattern":""}`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(root, ".evolve", "bad-verdict-baseline.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

func TestC1439_006_SalvageReportFoldsBaselineViaCLI(t *testing.T) {
	t.Parallel()
	bin := buildEvolve(t)
	proj := writeBaselineProject(t)

	stdout, stderr, code, err := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if err != nil || code != 0 {
		t.Fatalf("evolve salvage report -json: exit=%d err=%v\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}

	var got struct {
		Total       int            `json:"total"`
		Recoverable int            `json:"recoverable"`
		Rate        float64        `json:"rate"`
		ByPattern   map[string]int `json:"by_pattern"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", err, stdout)
	}
	if got.Total != 3 {
		t.Errorf("total = %d, want 3 — the foreign event_type and the blank lines must not enter the denominator", got.Total)
	}
	if got.Recoverable != 2 {
		t.Errorf("recoverable = %d, want 2", got.Recoverable)
	}
	if math.Abs(got.Rate-2.0/3.0) > 1e-9 {
		t.Errorf("rate = %v, want %v (recoverable/total)", got.Rate, 2.0/3.0)
	}
	if got.ByPattern["trailing-comma"] != 1 || got.ByPattern["fenced-json"] != 1 {
		t.Errorf("by_pattern = %v, want trailing-comma:1 fenced-json:1 — the fenced-json count must come from the "+
			"bad_verdict record, never from the foreign emitter's line", got.ByPattern)
	}
	if _, phantom := got.ByPattern[""]; phantom {
		t.Errorf("by_pattern carries an empty-string bucket %v — a non-recoverable record has no pattern and must "+
			"not manufacture a phantom shape", got.ByPattern)
	}

	repo := acsassert.RepoRoot(t)
	for _, rel := range []string{"go/cmd/evolve/cmd_salvage.go", "go/internal/deliverable/salvage_report.go"} {
		if _, _, c, _ := acsassert.SubprocessOutput("git", "-C", repo, "ls-files", "--error-unmatch", rel); c != 0 {
			t.Errorf("%s is untracked — it will be dropped at ship", rel)
		}
	}
}

func TestC1439_007_SalvageReportFailsLoudlyOnTornRecord(t *testing.T) {
	t.Parallel()
	bin := buildEvolve(t)
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	torn := `{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"trailing-comma"}` + "\n" +
		`{"event_type":"bad_verdict_class` + "\n"
	if err := os.WriteFile(filepath.Join(proj, ".evolve", "bad-verdict-baseline.jsonl"), []byte(torn), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, _ := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if code == 0 {
		t.Fatalf("exit=0 on a torn record — a silently skipped line biases the measured rate.\nstdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "bad-verdict-baseline.jsonl") || !strings.Contains(stderr, "line 2") {
		t.Errorf("stderr does not name the sidecar file and the offending line (want %q + %q): %q",
			"bad-verdict-baseline.jsonl", "line 2", firstLine(stderr))
	}
}

func TestC1439_008_SalvageIsInTheDispatchTable(t *testing.T) {
	t.Parallel()
	bin := buildEvolve(t)

	stdout, stderr, _, err := acsassert.SubprocessOutput(bin, "help")
	if err != nil {
		t.Fatalf("evolve help: %v", err)
	}
	if !strings.Contains(stdout+stderr, "salvage") {
		t.Errorf("`salvage` is absent from the top-level command listing — it is not wired into registry.go's "+
			"dispatch table, so no operator can reach the reader.\n%s", stdout+stderr)
	}

	_, _, code, _ := acsassert.SubprocessOutput(bin, "salvage")
	if code == 0 {
		t.Errorf("`evolve salvage` with no subcommand exited 0 — it must reject and print usage, not silently " +
			"default to a behaviour")
	}
	_, _, code, _ = acsassert.SubprocessOutput(bin, "salvage", "nonesuch")
	if code == 0 {
		t.Errorf("`evolve salvage nonesuch` exited 0 — an unknown subcommand must be rejected")
	}
}

func TestC1439_009_NamedGuardAndApicoverTestsPass(t *testing.T) {
	t.Parallel()
	root := acsassert.RepoRoot(t)
	const pattern = "TestClassifyBadVerdict_UnmatchedBacktickFalsePositive|" +
		"TestClassifyBadVerdict_QuotedEchoStillSuppressed|" +
		"TestClassifyBadVerdict_BacktickAtContentBoundary|" +
		"TestSummarizeBadVerdictBaseline_NamesAndExercises"

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", pattern, "./internal/deliverable")
	if err != nil || code != 0 {
		t.Fatalf("go test -run %q ./internal/deliverable: exit=%d err=%v\n%s\n%s", pattern, code, err, stdout, stderr)
	}
	if strings.Contains(stdout, "no tests to run") {
		t.Fatalf("exit 0 but NO test matched — none of the required guard/apicover tests exist:\n%s", stdout)
	}
	for _, name := range []string{
		"TestClassifyBadVerdict_UnmatchedBacktickFalsePositive",
		"TestClassifyBadVerdict_QuotedEchoStillSuppressed",
		"TestClassifyBadVerdict_BacktickAtContentBoundary",
		"TestSummarizeBadVerdictBaseline_NamesAndExercises",
	} {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not run and PASS in internal/deliverable — the landing owes this test", name)
		}
	}
}
