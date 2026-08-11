//go:build acs

// Package cycle1439 materialises the acceptance criteria for this lane's single
// fleet-scoped task, `salvage-worktree-relanding` (triage-report.md ## top_n).
//
// What this cycle is. Not new design: a LANDING. The stranded worktree
// .evolve/worktrees/cycle-42824668-1407 (branch cycle-42824668-1407, snapshot
// 04d3dee1, continuation records `task-a-salvage-extraction-stage` /
// `task-b-decoy-sentinel-fixture`) holds complete, never-landed work — the
// quote-aware + tail-anchored `ownSentinelPayload` selector, the
// `SummarizeBadVerdictBaseline` reader, and the `evolve salvage report` CLI —
// blocked on one isolated correctness defect: `isQuotedEcho` treats a single
// adjacent backtick as proof of a CLOSED inline-code span, so one stray
// unmatched backtick suppresses a report's own genuine verdict sentinel
// (cycle-1407 adversarial finding F1).
//
// Predicate strategy. Every predicate exercises the system: predicates 001-005
// call `deliverable.ClassifyBadVerdict` directly and assert on its returned
// classification; 006-008 build and drive the REAL CLI entry point
// (go/cmd/evolve, via the registry.go dispatch table) as a subprocess and assert
// on its emitted JSON / exit codes; 009 runs the named unit + apicover tests in
// internal/deliverable. No predicate here is load-bearing on a source grep —
// the cycle-85 degenerate-predicate ban.
//
// Wiring proof, not unit proof. 006-008 deliberately reach the salvage reader
// through `evolve salvage report`, never by calling SummarizeBadVerdictBaseline
// directly: a reader whose only caller is a test is dead code, and the whole
// point of this landing is that the sidecar written since cycle-1389 finally has
// a production reader an operator can run.
//
// RED baseline (this worktree, main-based). 002/003 fail because today's
// ClassifyBadVerdict takes the FIRST sentinel-shaped span with no quote
// awareness at all; 006/007/008 fail because go/cmd/evolve/cmd_salvage.go and
// go/internal/deliverable/salvage_report.go do not exist on main; 009 fails
// because the named guard/apicover tests are worktree-only. 001/004/005 are
// pre-existing GREEN and are pinned as regression guards: they are exactly the
// cases a naive fix ("drop quote-awareness" / "last-match-wins only") would
// break, so they must stay green THROUGH the landing.
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

// --- shared fixture text -----------------------------------------------------

// malformedTailSentinel is a report's OWN verdict, malformed in the single most
// common LLM way (trailing comma) — the shape ClassifyBadVerdict already claims
// as SalvagePatternTrailingComma recoverable.
const malformedTailSentinel = "<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"FAIL\",\"schema_version\":2,} -->\n"

// cleanTailSentinel parses cleanly: a report carrying only this has nothing to
// salvage, so Recoverable MUST be false.
const cleanTailSentinel = "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":2} -->\n"

// quotedRecoverableDecoy is a sentinel a report merely QUOTES while discussing
// the contract — balanced inline-code backticks on both sides. Its payload is
// trailing-comma malformed, so a classifier without quote-awareness will report
// it as this report's own recoverable verdict.
const quotedRecoverableDecoy = "The contract shape is " +
	"`<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":1,} -->` " +
	"— note the stray comma an agent often leaves behind.\n"

// quotedUnrecoverableDecoy is a quoted echo whose payload is malformed in a way
// the classifier does NOT claim as recoverable (a missing separator, not a
// trailing comma). A classifier that keys off this span reports "not
// recoverable" and never reaches the report's own tail sentinel.
const quotedUnrecoverableDecoy = "Another phase emitted " +
	"`<!-- evolve-verdict: {\"phase\":\"audit\" \"verdict\":\"FAIL\"} -->` " +
	"which the strict parser rejected outright.\n"

// --- 001-005: classifier behaviour (direct calls) ----------------------------

// TestC1439_001_UnmatchedBacktickDoesNotSuppressOwnSentinel is finding F1 itself.
//
// One stray, never-closed backtick sits immediately before the report's own
// malformed tail sentinel. Adjacency alone must NOT be read as a quoted echo:
// the span is the report's genuine verdict and is plainly recoverable.
//
// Pre-existing GREEN on main (which has no quote-awareness at all) and RED in
// the stranded worktree. It is pinned here because the landing introduces
// isQuotedEcho, and the ONLY acceptable landing is one where this stays green —
// i.e. the closure requirement, not a revert of quote-awareness.
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

// TestC1439_002_QuotedDecoyIsNotTheReportsOwnVerdict is the quote-awareness
// half. A malformed sentinel wrapped in BALANCED inline-code backticks is prose
// quoting the contract; the report's own verdict, further down, parses cleanly.
// There is therefore nothing to salvage.
//
// RED on main: ClassifyBadVerdict takes the FIRST sentinel-shaped span, which is
// the decoy, and reports it recoverable.
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

// TestC1439_003_RealTailSentinelClassifiesThroughQuotedDecoy is the other
// direction, and the crux of the landing: the decoy above is quoted AND
// unrecoverably malformed, while the report's own tail sentinel below it carries
// a trailing comma. The classifier must skip the echo and classify the real one.
//
// RED on main: first-match-wins keys off the decoy and returns not-recoverable,
// so a genuinely salvageable report is counted against the baseline rate the
// extraction stage is gated on.
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

// TestC1439_004_QuotedDecoyAfterRealSentinelIgnored is the adversarial guard
// against the cheap fix. "Last match wins" alone passes 003 while failing here:
// the real, clean sentinel comes FIRST and a malformed decoy is quoted BELOW it.
// Only quote-awareness plus tail anchoring passes 002, 003 and 004 together.
//
// Pre-existing GREEN on main (first-match-wins gets this right by accident); it
// is the negative test that keeps the landing from regressing into last-wins.
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

// TestC1439_005_BacktickAtContentBoundaries is the edge/OOD axis: a sentinel
// flush against offset 0 and one flush against len(content)-1 with a trailing
// backtick. Any adjacency check that peeks at content[start-1] / content[end]
// without bounds-guarding panics here (index out of range), and a panic in a
// pure classifier takes down the phase that called it.
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

// --- CLI harness -------------------------------------------------------------

// buildEvolve compiles go/cmd/evolve — the real CLI entry point whose registry.go
// dispatch table must carry the `salvage` subcommand — and returns the binary
// path. One named package, never a `./...` sweep (flaky-predicate-shape rule),
// and `go -C` rather than a bare `go` so the repo resolves from the worktree and
// not from whatever cwd the fleet lane happens to have.
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

// writeBaselineProject materialises a project root holding the JSONL sidecar the
// reader folds. Three bad_verdict_classified records (two recoverable), plus one
// FOREIGN event and one blank line — both of which the reader must skip without
// touching the denominator.
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

// firstLine keeps a failure message readable when the CLI answers with its full
// usage dump (the shape of the RED baseline, where `salvage` is unregistered).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

// --- 006-008: CLI reachability (wiring proofs) -------------------------------

// TestC1439_006_SalvageReportFoldsBaselineViaCLI drives the reader through its
// PRODUCTION caller — `evolve salvage report -json`, dispatched by registry.go —
// and asserts the folded summary. Calling SummarizeBadVerdictBaseline directly
// would pass on dead code; the whole point of the landing is that the sidecar
// finally has an operator-reachable reader.
//
// RED on main: `salvage` is not in the dispatch table, so the CLI exits non-zero
// with an unknown-command error and emits no JSON at all.
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

	// Auxiliary (not load-bearing): the landed sources must be git-TRACKED, not
	// merely present on disk — an untracked file is silently dropped at ship.
	repo := acsassert.RepoRoot(t)
	for _, rel := range []string{"go/cmd/evolve/cmd_salvage.go", "go/internal/deliverable/salvage_report.go"} {
		if _, _, c, _ := acsassert.SubprocessOutput("git", "-C", repo, "ls-files", "--error-unmatch", rel); c != 0 {
			t.Errorf("%s is untracked — it will be dropped at ship", rel)
		}
	}
}

// TestC1439_007_SalvageReportFailsLoudlyOnTornRecord is the negative axis. The
// sidecar is append-per-emit, so a killed process can leave a torn line. A
// summarizer that skipped it would under-count the denominator and bias the very
// rate it exists to measure (rule 12: fail loudly). The CLI must exit non-zero
// and name the file and line.
func TestC1439_007_SalvageReportFailsLoudlyOnTornRecord(t *testing.T) {
	t.Parallel()
	bin := buildEvolve(t)
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	torn := `{"event_type":"bad_verdict_classified","recoverable":true,"pattern":"trailing-comma"}` + "\n" +
		`{"event_type":"bad_verdict_class` + "\n" // killed mid-append
	if err := os.WriteFile(filepath.Join(proj, ".evolve", "bad-verdict-baseline.jsonl"), []byte(torn), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code, _ := acsassert.SubprocessOutput(bin, "salvage", "report", "-json", "-project-root", proj)
	if code == 0 {
		t.Fatalf("exit=0 on a torn record — a silently skipped line biases the measured rate.\nstdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "bad-verdict-baseline.jsonl") || !strings.Contains(stderr, "line 2") {
		// Truncated: an unwired CLI answers with its whole usage dump, which
		// would bury every other predicate's output in the RED log.
		t.Errorf("stderr does not name the sidecar file and the offending line (want %q + %q): %q",
			"bad-verdict-baseline.jsonl", "line 2", firstLine(stderr))
	}
}

// TestC1439_008_SalvageIsInTheDispatchTable pins the seam itself: `salvage` must
// be a registered subcommand reachable from the top-level CLI, not merely a
// function that exists. An unregistered runSalvage is dead code.
//
// It also pins the usage contract — a bare `salvage` with no subcommand must
// fail rather than default to something — so the surface cannot silently widen.
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

// --- 009: named guard + apicover coverage ------------------------------------

// TestC1439_009_NamedGuardAndApicoverTestsPass runs the unit tests the landing
// owes, by name, in ONE package with a narrowed -run (never a ./... sweep):
//
//   - the three isQuotedEcho guard cases the build plan enumerates, and
//   - the apicover named test that exercises the newly exported
//     SummarizeBadVerdictBaseline / BaselineSummary. internal/deliverable is
//     already enrolled in go/.apicover-enforce:237, so every exported symbol the
//     landing adds must be named in a real assertion or the repo-wide apicover
//     gate (ADR-0069's second gate) fails the build.
//
// Asserting on `--- PASS: <name>` lines, and rejecting "no tests to run", is
// what makes this a coverage proof rather than a vacuous exit-0.
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
