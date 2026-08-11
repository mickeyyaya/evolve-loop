//go:build acs

// Package cycle1407 materialises the acceptance criteria for this lane's two
// fleet-assigned tasks, both inside go/internal/deliverable:
//
//   - salvage-baseline-rate-report        (fleet id task-a-salvage-extraction-stage)
//   - salvage-classifier-quoted-decoy-case (fleet id task-b-decoy-sentinel-fixture)
//
// # Task A — why a summarizer, and why a CLI caller
//
// salvage_instrument.go has been WRITING .evolve/bad-verdict-baseline.jsonl
// since cycle-1389, and nothing has ever read it: grepping the tree for
// `bad-verdict-baseline` outside the writer and its own tests returns only the
// writer. The salvage-layer portfolio item gates its extraction/coercion stage
// on a measured recoverable-malformed RATE, so the gate is currently blocked on
// a number no code computes. This lane computes it.
//
// The summarizer is a pure function over an io.Reader, but a pure function
// whose only caller is a test is dead code (house rule 2 — a wiring proof is a
// REACHABILITY test, not a unit test). Predicate 003 therefore drives the REAL
// `evolve` binary end to end and 004 executes the invocation the docs publish.
// Scout listed a "CLI-facing surface" for Task 1 while triage's files={} set
// named only internal/deliverable + docs; the triage set is evidence, not an
// allowlist, so go/cmd/evolve/ is in scope for this lane — without it there is
// no production caller and 003 can never go green.
//
// # Task B — the fixture already passes, for the WRONG reason
//
// The scout AC ("quoted sentinel examples in prose must not register as
// Recoverable") was probed against HEAD before this contract was frozen:
//
//	ClassifyBadVerdict(cycle1298-quoted-decoys.md)
//	  -> recoverable=false pattern="" reason="evolve-verdict sentinel present but
//	     its payload is not recoverably malformed"
//
// Recoverable=false — so the literal AC is pre-existing GREEN (pinned by 005).
// But the REASON proves the classifier never reached the report's own tail
// sentinel: sentinelPayloadRE.FindStringSubmatch takes the FIRST match in the
// document, and the first match in that fixture is a QUOTED DECOY — an
// other-phase sentinel the auditor echoed into prose while describing the
// cycle-1298 F-1 bypass. The classifier reproduces, inside itself, the exact
// first-sentinel-wins defect the fixture was landed to document, and the exact
// class `.evolve/instincts/lessons/cycle-641-...yaml` names ("classifiers MUST
// exclude any span that is a verbatim echo of injected prompt/instruction
// text"). So the real, RED-able criterion is DECOY IMMUNITY, and 006 is the
// crux: with a genuinely malformed tail sentinel appended to the real corpus,
// the classifier must classify from THAT sentinel, not from a quoted decoy.
//
// 007 is the anti-overcorrection guard: "take the LAST match instead of the
// first" also satisfies 006, and is still wrong. A decoy quoted AFTER the real
// sentinel must be ignored too, so the fix has to be genuine quote-awareness.
//
// # Predicate strategy
//
// Every predicate calls the system under test and asserts on a return value,
// a process exit code, or emitted output. None is a source-grep of production
// code (the cycle-85 degenerate-predicate ban). 008's file-content half is a
// single-source-of-truth check (no re-typed fixture) that rides on top of an
// executed `go test` run whose named subtest must actually report PASS — a
// `-run` pattern matching nothing also exits 0, so the "--- PASS:" line is what
// rules the no-op out.
//
// Reachability probe (cycle-644 obligation, run before freezing 003/004's
// package-qualified pin): `cmd/evolve` importing `internal/deliverable` was
// compiler-probed with a throwaway blank import — `go build ./cmd/evolve/`
// exit 0. The import is buildable, not merely plausible.
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

// decoyFixtureRel is the real cycle-1298 adversarial-review report: 5 quoted
// sentinel decoys in prose plus the report's own tail sentinel. It is read from
// its ONE canonical location under phasecontract/testdata — never re-typed —
// so this suite and phasecontract's sentinel_tailanchor_test.go stay bound to
// the same bytes.
const decoyFixtureRel = "go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md"

// regressionTestRel is where Task B's subtest must land (triage files={}).
const regressionTestRel = "go/internal/deliverable/salvage_instrument_test.go"

// readDecoyFixture returns the fixture bytes, failing loudly if the corpus this
// whole task is defined against has moved.
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

// acsSubprocess wraps SubprocessOutput so a missing toolchain skips instead of
// red-failing on a bare export (same guard as cycle-1156's predicates).
func acsSubprocess(t *testing.T, name string, args ...string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(name, args...)
	if code == -1 && err != nil && strings.Contains(err.Error(), "not found") {
		t.Skipf("%s not available: %v", name, err)
	}
	return stdout, stderr, code
}

// goPkg resolves a package directory to an ABSOLUTE path. SubprocessOutput
// exposes no Dir, so the child inherits this test binary's cwd
// (go/acs/cycle1407) — against which a relative "./internal/deliverable" or
// "../../cmd/evolve" silently resolves somewhere else in a worktree, a fleet
// lane, or the main tree. Absolute paths off RepoRoot are the cwd-independent
// form the flaky-predicate-shape rules require.
func goPkg(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go", rel)
}

// baselineLine renders one JSONL record in the exact shape
// log.SidecarWriter.EmitAbnormal emits (events.go:72-84): canonical event_type
// and timestamp keys, then the caller's Fields flattened alongside them.
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

// seedBaseline writes a project root containing .evolve/bad-verdict-baseline.jsonl
// with a HAND-COUNTED mix: 5 bad_verdict_classified records, 3 of them
// recoverable (2 trailing-comma, 1 fenced-json), 2 not — plus one foreign event
// that must NOT be counted. Expected: Total=5, Recoverable=3, Rate=0.6.
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
		// Foreign event sharing the sidecar: must be ignored entirely, not
		// counted as a non-recoverable bad verdict (which would silently
		// deflate the very rate this instrumentation exists to measure).
		baselineLine(t, "phase_started", "", false),
	}
	p := filepath.Join(root, ".evolve", "bad-verdict-baseline.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write seeded baseline: %v", err)
	}
	return root
}

// --- Task A: salvage-baseline-rate-report -----------------------------------

// AC-A1 (RED, behavioural core). A pure summarizer over the baseline JSONL
// computes the recoverable-malformed rate and the per-pattern breakdown.
//
// Hand-computed against seedBaseline's 6 lines: 5 bad_verdict_classified
// records (the phase_started line is foreign and must be skipped), 3 of them
// Recoverable, so Rate == 0.6 exactly; ByPattern counts only the patterns that
// actually appeared, so the empty pattern of the two non-recoverable records
// must not manufacture a phantom bucket.
//
// Per the Research → Implementation Map, counts are keyed BY PATTERN precisely
// so a future reader cannot conflate distinct malformation shapes into one
// undifferentiated "bad output" number.
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

// AC-A2 (RED, negative + edge). Two boundaries the rate must survive, because
// both WILL occur in production: an empty baseline on a fresh project root, and
// a torn/truncated JSONL line (the sidecar appends per-emit; a killed process
// can leave a partial line).
//
//   - empty input: Total 0 and Rate 0 — never NaN from a 0/0 division. A NaN
//     serialises to invalid JSON and would poison the gate's own report.
//   - malformed line: a LOUD error, never a silently-shortened count. Rule 12
//     (fail loudly) applies with force here: a summarizer that skips unparseable
//     lines under-reports the denominator and biases the measurement it exists
//     to produce.
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
	if empty.Rate != empty.Rate { // NaN != NaN
		t.Errorf("empty baseline Rate is NaN — 0/0 was evaluated unguarded")
	}

	torn := baselineLine(t, "bad_verdict_classified", "fenced-json", true) + "\n{\"event_type\":\"bad_verd"
	if _, err := deliverable.SummarizeBadVerdictBaseline(strings.NewReader(torn)); err == nil {
		t.Errorf("a truncated JSONL line was accepted silently: want a loud error, because dropping unparseable " +
			"lines under-counts the denominator and biases the recoverable-malformed rate")
	}
}

// AC-A3 (RED, CRUX — the wiring/reachability proof). The rate must be reachable
// from the REAL production entry point, not merely from this suite.
//
// This drives the actual `evolve` binary: `evolve salvage report -json` against
// a project root whose seeded baseline has the same hand-computed 0.6 rate as
// 001. A summarizer wired into nothing is dead code and this predicate stays
// RED until go/cmd/evolve/cmd_salvage.go plus its go/cmd/evolve/registry.go row
// exist — Builder must name that caller file:line in build-report.md.
//
// Deliberately NOT a `-run` of some unit test: calling the seam directly would
// pass on dead code and prove nothing (house rule 2).
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

// AC-A4 (RED, docs — and executable rather than grepped). operating-policy 3.8
// requires the README to carry an issue/gap/solution paragraph, not a changelog
// line. A FileContains check on prose is gameable by pasting the string, so
// this predicate instead EXTRACTS the invocation the README publishes and RUNS
// it: documentation that does not execute fails.
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

// --- Task B: salvage-classifier-quoted-decoy-case ---------------------------

// AC-B1 (pre-existing GREEN — pinned deliberately). The scout AC as literally
// written: the cycle-1298 corpus, whose 5 quoted sentinels are prose echoes,
// must not be classified Recoverable.
//
// Probed GREEN on HEAD before this contract was frozen, and recorded as such in
// test-report.md rather than presented as new work. It is kept because it is
// the property a fix to 006/007 could most plausibly break: an implementation
// that starts trusting sentinels more eagerly regresses right here.
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

// AC-B2 (RED, CRUX — decoy immunity, false-negative direction). The report's
// OWN verdict is the tail sentinel; the quoted ones are evidence being
// discussed. Append a genuinely malformed tail sentinel — valid sentinel shape,
// payload with a trailing comma before the closing brace — to the real corpus,
// and the classifier must recover it as SalvagePatternTrailingComma.
//
// On HEAD this is RED: sentinelPayloadRE.FindStringSubmatch returns the FIRST
// match in the document, which in this corpus is a QUOTED decoy, so
// ClassifyBadVerdict returns early with
// "sentinel present but its payload is not recoverably malformed" (probed) and
// never reaches the real tail sentinel at all. Every decoy here is real fixture
// content — only the genuine tail sentinel is appended.
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

// AC-B3 (negative / anti-overcorrection guard). "Take the LAST sentinel instead
// of the first" also satisfies 006 and is still wrong, so this pins the other
// direction: a decoy quoted AFTER the report's real tail sentinel must be
// ignored too. The corpus already ends with its own well-formed sentinel; we
// append a backticked, clearly-quoted malformed one as ordinary prose.
//
// Correct behaviour is Recoverable=false — the real verdict parsed fine, and
// nothing about a quoted example makes the report salvageable. A last-wins
// implementation returns trailing-comma here and fails.
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

// AC-B4 (RED, deliverable-existence + executed). Task B's deliverable is a
// regression case landed in the package's own test file, so the property is
// guarded by `go test ./internal/deliverable` forever — not only by this
// cycle-scoped ACS suite, which is never replayed.
//
// Two halves, both load-bearing:
//   - the subtest must reference the fixture by PATH (single-source-of-truth:
//     a re-typed copy of 266 lines would drift silently from the corpus that
//     phasecontract's own tests pin);
//   - the named test must actually RUN and report PASS. A `-run` pattern that
//     matches nothing exits 0, so the "--- PASS:" line is the anti-no-op check.
//
// Scoped to ONE named package, never a ./... sweep (flaky-predicate-shape).
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

// AC-A5 (RED, house-rule floor — apicover graduation). ./internal/deliverable is
// already enrolled in go/.apicover-enforce (line 232), so the repo-wide ADR-0069
// gate requires every NEW exported symbol to be named AND exercised in
// internal/deliverable/apicover_named_test.go. Task A adds
// SummarizeBadVerdictBaseline and BaselineSummary; omitting them aborts a later
// build phase with an unenrolled-symbol failure that reads as unrelated.
//
// Executed, not grepped: this runs the package's own apicover gate for the
// enrolled package and requires exit 0.
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

	// Reproduce CI's exact invocation (.github/workflows/go.yml:118-128):
	// `apicover -enforce -cover coverage.func.txt <dir>`. The -cover profile is
	// NOT optional — flagless, every named-but-unmeasured symbol reads as
	// false-green (probed: 22 pre-existing false-greens, all of which vanish
	// once the profile is supplied). A predicate that omitted it would demand
	// Builder fix 22 unrelated symbols to go green: unsatisfiable by design,
	// which is the cycle-644 trap this contract must not re-set.
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
