//go:build acs

// Package cycle1685 materialises the cycle-1685 acceptance criteria for the one
// fleet-scoped inbox id `evalgate-selectedslugs-nil-blindness` (scout task
// `evalgate-parse-miss-vs-convergence-signal`).
//
// The defect. `evalgate.SelectedSlugs` collapses two categorically different
// zero-slug scout-report shapes into the same `nil`:
//
//  1. genuine convergence — no "## Selected Tasks" section at all (nothing was
//     claimed; fail-open is CORRECT here), and
//  2. format drift — a "## Selected Tasks" section that IS present with real
//     task prose whose slug is stated in a form `slugLineRE` does not
//     recognise, so it parses to zero slugs (the cycle-1570 shape: scout
//     selected `config-gate-default-policy-authority`, no eval file, and Gate
//     A's fail-open path let it through to surface three phases later as an
//     audit H1).
//
// Gate A's `check()` — the only seam an agent or operator actually sees —
// returns ("", false) for BOTH, so nothing distinguishes "nothing to check"
// from "something to check that we failed to read".
//
// The accepted fix is a new exported `SelectedTasksParseMiss(report string) bool`
// scoped strictly to the bounded "## Selected Tasks" section, surfaced through
// Gate A as an ADVISORY WARN. It is deliberately NOT a new hard block: blocking
// every zero-slug report would false-block every genuine convergence cycle and
// contradict the package's own documented fail-open-on-ambiguity contract.
//
// Predicate strategy — every predicate exercises the system under test (the
// cycle-85 degenerate-predicate ban):
//
//   - 001-005 CALL the detector on real report bodies and assert its boolean.
//     001 is the crux positive; 002/003/004/005 are the negatives and the
//     section-bounding edges that a no-op `return true` would fail.
//   - 006 is the CALLER PROOF: it drives the real production reviewer
//     (`evalgate.NewReviewer(...).Review(...)`, the core.WithReviewer seam) and
//     asserts the advisory actually reaches Gate A's emitted log line. A
//     detector nothing calls from production is dead code, and a predicate that
//     only calls it directly would pass on dead code.
//   - 007 pins the contract the fix must NOT break: silence on genuine
//     convergence, and the pre-existing HARD BLOCK on a selected slug with no
//     eval file still fires at enforce.
//   - 008 closes the apicover false-green hole (internal/evalgate is enrolled
//     in go/.apicover-enforce) using apicover's own AST detector.
//   - 009 proves the durable eval's three `[code]` grader tests actually RAN
//     and passed — `go test -run <name>` that matches NOTHING still exits 0,
//     which is the vacuous-pass hole those graders would otherwise carry — and
//     that the files carrying them are git-TRACKED (a gitignored test file is
//     dropped at ship: the cycle-92 shape).
//   - 010 is the no-regression floor for the package.
//   - 011-012 materialise the standing audit finding M1 (audit round 1). They are
//     the ONE class in this file that asserts on a prose deliverable, because the
//     remedy M1 asks for IS prose: the explanation document must state the
//     advisory's aggregate measured operating point instead of the marginal
//     contribution of one formatting variant. They carry an explicit
//     `// acs-predicate: config-check` waiver and are NOT bare greps — 011 parses
//     the numerator/denominator/percentage out of the document and re-derives the
//     arithmetic, so pasting the pre-existing `12 of 84` / `one cycle in seven`
//     figures cannot satisfy them.
package cycle1685

import (
	"context"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// cycle1570Report is the REAL incident shape, reproduced verbatim from this
// cycle's bug-reproduction phase: a "## Selected Tasks" section holding genuine
// task prose for `config-gate-default-policy-authority`, whose slug is stated as
// free prose ("Task slug: ...") rather than the "- **Slug:** <kebab>" bullet the
// parser requires. Neither empty nor absent — and it parses to zero slugs.
const cycle1570Report = "# Scout Report\n\n## Selected Tasks\n\n" +
	"### Task 1: Config gate default policy authority\n" +
	"Task slug: config-gate-default-policy-authority\n" +
	"- **Type:** bug\n- **Complexity:** S\n\n" +
	"No eval file was authored for this task; the slug is only named in prose above.\n"

// evalgatePkgDir is the package under test, resolved from the repo root.
func evalgatePkgDir(root string) string { return filepath.Join(root, "go", "internal", "evalgate") }

// TestC1685_001_ParseMissTrueOnCycle1570Shape is the crux: the detector must
// return true for a Selected Tasks section that has real content the parser
// could not read. Asserts the fixture PREMISE first (this shape really does
// still parse to zero slugs) so a future parser change that stops reproducing
// the incident fails loudly here instead of the predicate quietly testing
// nothing.
func TestC1685_001_ParseMissTrueOnCycle1570Shape(t *testing.T) {
	if got := evalgate.SelectedSlugs(cycle1570Report); got != nil {
		t.Fatalf("fixture premise broken: the cycle-1570 shape must still parse to ZERO slugs for this predicate to exercise the reported bug; SelectedSlugs()=%v", got)
	}
	if !evalgate.SelectedTasksParseMiss(cycle1570Report) {
		t.Errorf("SelectedTasksParseMiss(cycle-1570 shape)=false, want true — a '## Selected Tasks' section carrying real, unparseable task prose is still indistinguishable from a genuine convergence report, which is the whole defect")
	}
}

// TestC1685_002_ParseMissFalseOnGenuineConvergence is the primary negative: a
// report that claims no work must NOT be flagged. A detector that returns true
// here would fire on every converged cycle and reintroduce exactly the
// false-blocking risk the fail-open contract exists to avoid.
func TestC1685_002_ParseMissFalseOnGenuineConvergence(t *testing.T) {
	cases := []struct{ name, report string }{
		{"no Selected Tasks section at all", "## Gap Analysis\nNothing to do.\n"},
		{"empty report", ""},
		{"decision trace with zero selections", "## Decision Trace\n```json\n{\"decisionTrace\":[]}\n```\n"},
		{"prose report that never uses the heading", "# Scout Report\n\nThe backlog converged; no task was selected this cycle.\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if evalgate.SelectedTasksParseMiss(c.report) {
				t.Errorf("SelectedTasksParseMiss()=true, want false — a genuine convergence report must not be reported as format drift")
			}
		})
	}
}

// TestC1685_003_ParseMissFalseOnWellFormedSelections pins the healthy path: a
// section the parser CAN read is not drift. The last case is the eval's explicit
// negative — a malformed Decision Trace block must not leak into a signal that
// is scoped to the Selected Tasks section only.
func TestC1685_003_ParseMissFalseOnWellFormedSelections(t *testing.T) {
	cases := []struct{ name, report string }{
		{
			name: "selected-tasks prose",
			report: "## Selected Tasks\n\n### Task 1: Cache\n- **Slug:** add-cache\n- **Type:** feature\n\n" +
				"### Task 2: Limit\n- **Slug:** rate-limit\n\n## Deferred\n- something\n",
		},
		{
			name:   "section bounded by the next heading",
			report: "## Selected Tasks\n- **Slug:** in-section\n\n## Deferred\n- **Slug:** not-counted\n",
		},
		{
			name: "both sources union",
			report: "## Selected Tasks\n- **Slug:** add-cache\n\n## Decision Trace\n```json\n" +
				"{\"decisionTrace\":[{\"slug\":\"trace-only\",\"finalDecision\":\"selected\"}]}\n```\n",
		},
		{
			name: "malformed decision trace beside a readable section",
			report: "## Selected Tasks\n- **Slug:** ok-slug\n\n## Decision Trace\n```json\n" +
				"{not valid json\n```\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evalgate.SelectedSlugs(c.report); len(got) == 0 {
				t.Fatalf("fixture premise broken: this report is supposed to parse to at least one slug; SelectedSlugs()=%v", got)
			}
			if evalgate.SelectedTasksParseMiss(c.report) {
				t.Errorf("SelectedTasksParseMiss()=true, want false — the section parsed fine, so there is no drift to report")
			}
		})
	}
}

// TestC1685_004_ParseMissFalseOnContentlessSection is the boundary the scout
// acceptance calls out by name: a heading with only blank lines or comments
// under it is still convergence, not drift. A naive "heading present and zero
// slugs" implementation passes 001-003 and fails here.
func TestC1685_004_ParseMissFalseOnContentlessSection(t *testing.T) {
	cases := []struct{ name, report string }{
		{"heading then EOF", "## Selected Tasks\n"},
		{"heading with no trailing newline", "## Selected Tasks"},
		{"heading then blank lines only", "## Selected Tasks\n\n\n   \n\t\n"},
		{"heading then an html comment only", "## Selected Tasks\n\n<!-- none selected this cycle -->\n"},
		{"heading then blanks then the next heading", "## Selected Tasks\n\n\n## Deferred\n- carried prose\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if evalgate.SelectedTasksParseMiss(c.report) {
				t.Errorf("SelectedTasksParseMiss()=true, want false — an empty/comment-only section claims no work and must read as convergence, not as format drift")
			}
		})
	}
}

// TestC1685_005_ParseMissIsSectionBounded proves the signal honours the same
// "## " bound selectedTaskSlugs already uses: prose in a LATER section must not
// make an empty Selected Tasks section look like drift, and a drifted section
// must still be caught when another section follows it.
func TestC1685_005_ParseMissIsSectionBounded(t *testing.T) {
	outsideOnly := "## Selected Tasks\n\n## Deferred\n\n### Task 1: Elsewhere\n" +
		"Task slug: lives-in-another-section\nreal prose that is not in the bounded body\n"
	if evalgate.SelectedTasksParseMiss(outsideOnly) {
		t.Errorf("SelectedTasksParseMiss()=true for content that lives AFTER the next '## ' heading, want false — the signal must read only the bounded Selected Tasks body")
	}

	driftThenHeading := "## Selected Tasks\n\n### Task 1: Drifted\nTask slug: drifted-task\n" +
		"- **Type:** bug\n\n## Deferred\n- **Slug:** later-one\n"
	if got := evalgate.SelectedSlugs(driftThenHeading); got != nil {
		t.Fatalf("fixture premise broken: the bounded body must still yield zero slugs; SelectedSlugs()=%v", got)
	}
	if !evalgate.SelectedTasksParseMiss(driftThenHeading) {
		t.Errorf("SelectedTasksParseMiss()=false for a drifted section followed by another heading, want true — a trailing section must not mask the drift")
	}
}

// TestC1685_006_AdvisoryReachesGateAThroughTheProductionReviewer is the caller
// proof. It does NOT call the detector: it drives evalgate.NewReviewer — the
// object the orchestrator mounts at core.WithReviewer — and asserts the
// parse-miss actually surfaces on Gate A's emitted log line, which is the only
// channel an operator or agent ever sees. A detector wired into nothing would
// pass 001-005 and fail here.
func TestC1685_006_AdvisoryReachesGateAThroughTheProductionReviewer(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, cycle1570Report)

	var res core.ReviewResult
	logged := captureStderr(t, func() {
		res = evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
			Phase:       string(core.PhaseScout),
			ProjectRoot: root,
			Workspace:   ws,
		})
	})

	if !strings.Contains(logged, "evals-materialized") {
		t.Fatalf("Gate A emitted NO line for the cycle-1570 parse-miss shape — the advisory never reaches the production reviewer seam; captured stderr=%q", logged)
	}
	low := strings.ToLower(logged)
	if !strings.Contains(low, "selected tasks") && !strings.Contains(low, "parse-miss") && !strings.Contains(low, "parse miss") {
		t.Errorf("Gate A's advisory names neither the drifted section nor the parse-miss condition, so it is not actionable; captured stderr=%q", logged)
	}
	if !res.Approve {
		t.Errorf("Gate A REJECTED a parse-miss report (Approve=false, reason=%q) — the signal must be advisory; hard-blocking here false-blocks every genuine convergence cycle and contradicts the package's fail-open contract", res.Reason)
	}
}

// TestC1685_007_GateABlockingContractPreserved pins both halves of what the fix
// must not disturb: Gate A stays SILENT on a genuine convergence report (an
// advisory that fires every cycle is noise, not signal), and the pre-existing
// hard block on a selected slug with no eval file still fires at enforce.
func TestC1685_007_GateABlockingContractPreserved(t *testing.T) {
	t.Run("silent on genuine convergence", func(t *testing.T) {
		root, ws := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "## Gap Analysis\nNothing to do.\n")
		var res core.ReviewResult
		logged := captureStderr(t, func() {
			res = evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
				Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
			})
		})
		if strings.Contains(logged, "evals-materialized") {
			t.Errorf("Gate A emitted an advisory for a report that claims no work; captured stderr=%q", logged)
		}
		if !res.Approve {
			t.Errorf("Gate A rejected a genuine convergence report (reason=%q)", res.Reason)
		}
	})

	t.Run("still blocks a selected slug with no eval file", func(t *testing.T) {
		root, ws := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, "## Selected Tasks\n\n### Task 1: Thing\n- **Slug:** no-eval-authored\n")
		res := evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
			Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
		})
		if res.Approve {
			t.Fatalf("Gate A APPROVED a selected slug with no eval file on disk — the pre-existing blocking contract regressed")
		}
		if !strings.Contains(res.Reason, "no-eval-authored") {
			t.Errorf("rejection does not name the missing slug, so it is not actionable; reason=%q", res.Reason)
		}
	})
}

// TestC1685_008_NewExportIsNamedAndDocumented closes the apicover false-green
// hole: internal/evalgate is enrolled in go/.apicover-enforce, so an export that
// no test in the package NAMES reds `make apicover-enforce` for the whole tree.
// The check runs apicover's own AST detector rather than re-implementing it.
func TestC1685_008_NewExportIsNamedAndDocumented(t *testing.T) {
	root := acsassert.RepoRoot(t)
	if !acsassert.FileContains(t, filepath.Join(root, "go", ".apicover-enforce"), "./internal/evalgate") {
		t.Fatalf("internal/evalgate is no longer enrolled in go/.apicover-enforce — the premise of this predicate is gone; if enrollment was deliberately dropped, say so in build-report.md")
	}
	names, err := apicover.NamesReferencedInTests(context.Background(), evalgatePkgDir(root))
	if err != nil {
		t.Fatalf("apicover.NamesReferencedInTests(internal/evalgate): %v", err)
	}
	if !names["SelectedTasksParseMiss"] {
		t.Errorf("no _test.go in internal/evalgate names SelectedTasksParseMiss — an enrolled package's export that no test names is an apicover false-green and hard-fails `make apicover-enforce`")
	}
	doc, impl := regexp.MustCompile(`(?m)^// SelectedTasksParseMiss\b`), regexp.MustCompile(`(?m)^func SelectedTasksParseMiss\(`)
	var hasDoc, hasImpl bool
	for _, src := range packageSources(t, evalgatePkgDir(root)) {
		hasDoc = hasDoc || doc.MatchString(src)
		hasImpl = hasImpl || impl.MatchString(src)
	}
	if !hasImpl {
		t.Errorf("SelectedTasksParseMiss is not declared as a package-level func in internal/evalgate")
	}
	if !hasDoc {
		t.Errorf("SelectedTasksParseMiss carries no godoc comment opening with its own name — apicover's Phase-5 Definition of Done requires godoc on every export")
	}
}

// TestC1685_009_EvalGraderTestsRanPassedAndAreTracked proves the durable eval's
// three [code] graders are not vacuous. `go test -run <name>` whose pattern
// matches NOTHING still exits 0, so each grader is re-run here and the "--- PASS:
// <name>" line is required. It also pins the REAL incident slug as the fixture
// (not a synthetic stand-in, which the eval demands by name) and asserts the
// files carrying these tests are git-TRACKED — a gitignored test file is silently
// dropped at ship.
func TestC1685_009_EvalGraderTestsRanPassedAndAreTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, name := range []string{
		"TestSelectedTasksParseMiss",
		"TestCycle1570ReportShape",
		"TestSelectedTasksParseMiss_TraceOnlyReportNotFlagged",
		"TestMaterializationGate_ParseMissAdvisoryIsNonBlocking",
	} {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runIn(t, goDir, "go", "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/evalgate")
			if code != 0 {
				t.Fatalf("go test -run ^%s$ ./internal/evalgate exited %d\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
			}
			if !strings.Contains(stdout, "--- PASS: "+name) {
				t.Errorf("%s never RAN (exit 0 with no '--- PASS: %s' line) — a -run pattern that matches no test passes vacuously, so the eval's [code] grader for it would be a no-op", name, name)
			}
		})
	}

	carriers := sourcesNaming(t, evalgatePkgDir(root), "_test.go", "SelectedTasksParseMiss")
	if len(carriers) == 0 {
		t.Fatalf("no _test.go in internal/evalgate references SelectedTasksParseMiss")
	}
	incidentPinned := false
	for _, f := range carriers {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatalf("relativise %s: %v", f, err)
		}
		if _, _, code := runIn(t, root, "git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is UNTRACKED — a gitignored or unadded test file is dropped at ship, so the regression fixture would not survive the cycle", rel)
		}
	}
	for _, src := range packageSources(t, evalgatePkgDir(root)) {
		if strings.Contains(src, "config-gate-default-policy-authority") {
			incidentPinned = true
		}
	}
	if !incidentPinned {
		t.Errorf("the cycle-1570 report shape is not pinned by its real slug (config-gate-default-policy-authority) anywhere in internal/evalgate — the eval requires the actual defect shape, not a synthetic stand-in that can drift from it")
	}
}

// TestC1685_010_EvalgatePackageGreenAndGofmtClean is the no-regression floor:
// the whole package (including the pre-existing TestSelectedSlugs and
// TestSlugParserContract that pin the fail-open contract and the scout template
// tokens) stays green, and the tree stays formatted.
func TestC1685_010_EvalgatePackageGreenAndGofmtClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")

	stdout, stderr, code := runIn(t, goDir, "go", "test", "-count=1", "./internal/evalgate")
	if code != 0 {
		t.Errorf("go test -count=1 ./internal/evalgate exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	listed, _, code := runIn(t, goDir, "gofmt", "-l", "internal/evalgate")
	if code != 0 {
		t.Fatalf("gofmt -l internal/evalgate exited %d", code)
	}
	if strings.TrimSpace(listed) != "" {
		t.Errorf("gofmt -l internal/evalgate is not empty:\n%s", listed)
	}
}

// writeScoutReport places body at the artifact name the scout phase really
// writes, resolved from the phasecontract registry (the same SSOT Gate A reads
// through) so a registry rename cannot leave this predicate writing a file the
// gate no longer looks for.
func writeScoutReport(t *testing.T, workspace, body string) {
	t.Helper()
	name := phasecontract.ArtifactName(string(core.PhaseScout))
	if name == "" {
		t.Fatalf("phasecontract declares no artifact name for the scout phase")
	}
	if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what was
// written. reviewer.logf writes its gate lines to os.Stderr at call time, so this
// is how the production advisory is observed without re-implementing the gate.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	orig := os.Stderr
	os.Stderr = w
	func() {
		defer func() { os.Stderr = orig }()
		fn()
	}()
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// runIn executes one command with an explicit working directory — never the
// process cwd, which differs between the main tree, the cycle worktree and each
// fleet lane.
func runIn(t *testing.T, dir, name string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf strings.Builder
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %s %v in %s: %v", name, args, dir, err)
	}
	return outBuf.String(), errBuf.String(), code
}

// packageSources returns the contents of every .go file directly in dir.
func packageSources(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out = append(out, string(body))
	}
	return out
}

// sourcesNaming returns the paths of files in dir whose name ends with suffix
// and whose contents mention needle.
func sourcesNaming(t *testing.T, dir, suffix, needle string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if strings.Contains(string(body), needle) {
			out = append(out, p)
		}
	}
	return out
}

// The advisory's operating point, measured 2026-09-15 over
// `.evolve/runs/cycle-16*/scout-report.md` by calling the shipped detector on
// each report: 84 reports carry a "## Selected Tasks" section (all of them),
// SelectedTasksParseMiss fires on 59 of those (70.2%), and 56 of the 59 have an
// entirely empty SelectedSlugs union — i.e. Gate A really was checking nothing
// on those cycles, which is what makes the number a finding rather than noise.
// For context the pre-fix pattern fired on 71 of 84 (84.5%), so the backtick
// widening rescued exactly 12 reports (71-59), matching the document's own
// "12 of 84" claim.
const (
	measuredParseMissFires = 59
	measuredReportCorpus   = 84
	measuredEmptyUnion     = 56
)

// numOfDenRE matches a stated fraction in prose: "59 of 84", "59 of the 84",
// or "59/84".
var numOfDenRE = regexp.MustCompile(`(\d{1,4})\s*(?:/|of\s+(?:the\s+)?)\s*(\d{1,4})`)

// percentRE matches a stated percentage, with or without a decimal part.
var percentRE = regexp.MustCompile(`(\d{1,3}(?:\.\d+)?)\s*%`)

// isoDateRE matches the YYYY-MM-DD measurement stamp this diff already uses in
// slugs.go ("measured 2026-09-15").
var isoDateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// TestC1685_011_ExplanationStatesMeasuredOperatingPoint materialises standing
// audit finding M1. The explanation document must state the advisory's AGGREGATE
// operating point — the rate at which the signal actually speaks — not only the
// marginal contribution of the backticked-slug variant.
//
// acs-predicate: config-check
//
// Waiver rationale: the remedy M1 asks for is a prose correction to a tracked
// deliverable, so the document's text IS the system under test and there is no
// other seam to drive — the cycle-85 "magic string is not the fix" hazard does
// not apply, because here the string is precisely the fix. It is still not a
// bare grep: the numerator, denominator and percentage are parsed out of the
// document and the percentage is RE-DERIVED from the fraction, so the
// pre-existing "12 of 84 / one cycle in seven" figures cannot satisfy it.
func TestC1685_011_ExplanationStatesMeasuredOperatingPoint(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, doc := explanationDoc(t, root)

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relativise %s: %v", path, err)
	}
	if _, _, code := runIn(t, root, "git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is UNTRACKED — the corrected operating point would not survive the ship", rel)
	}

	var stated string
	for _, para := range splitParagraphs(doc) {
		for _, m := range numOfDenRE.FindAllStringSubmatch(para, -1) {
			if m[1] == strconv.Itoa(measuredParseMissFires) && m[2] == strconv.Itoa(measuredReportCorpus) {
				stated = para
			}
		}
	}
	if stated == "" {
		t.Fatalf("%s states the advisory's fire rate NOWHERE: no paragraph gives %d of %d.\n"+
			"M1: the document reasons about fire-rate noise using the marginal contribution of the\n"+
			"backtick variant (~1 cycle in 7) and never states the rate at which the advisory speaks.\n"+
			"Measured 2026-09-15 over .evolve/runs/cycle-16*/scout-report.md: %d of %d (%.1f%%), of\n"+
			"which %d have an entirely empty SelectedSlugs union.",
			rel, measuredParseMissFires, measuredReportCorpus,
			measuredParseMissFires, measuredReportCorpus,
			100*float64(measuredParseMissFires)/float64(measuredReportCorpus), measuredEmptyUnion)
	}

	// The percentage must be re-derivable from the fraction the document itself
	// states — a stated rate that does not match its own numerator/denominator is
	// the same class of misleading-but-individually-true number M1 is about.
	want := 100 * float64(measuredParseMissFires) / float64(measuredReportCorpus)
	var matched bool
	var seen []string
	for _, m := range percentRE.FindAllStringSubmatch(stated, -1) {
		seen = append(seen, m[1]+"%")
		if got, err := strconv.ParseFloat(m[1], 64); err == nil && math.Abs(got-want) <= 0.05 {
			matched = true
		}
	}
	if !matched {
		t.Errorf("the operating-point paragraph states no percentage consistent with its own %d of %d (want %.1f%%, found %v):\n%s",
			measuredParseMissFires, measuredReportCorpus, want, seen, stated)
	}
	if want <= 50 {
		t.Fatalf("premise broken: the measured rate %.1f%% is no longer a majority of cycles, so the present-tense framing this predicate requires is no longer the correct one — re-measure before editing the document", want)
	}
	if !strings.Contains(stated, "cycle-16") {
		t.Errorf("the operating-point paragraph does not name the corpus it was measured over (expected a cycle-16* scope), so the number cannot be re-derived:\n%s", stated)
	}
	if !isoDateRE.MatchString(stated) {
		t.Errorf("the operating-point paragraph carries no YYYY-MM-DD measurement date, so a future reader cannot tell what the number was true of (slugs.go already uses this convention):\n%s", stated)
	}

	// The empty-union subset is what makes the rate a finding rather than noise:
	// on those cycles Gate A was genuinely checking nothing.
	var unionStated bool
	for _, para := range splitParagraphs(doc) {
		if !strings.Contains(para, strconv.Itoa(measuredEmptyUnion)) {
			continue
		}
		low := strings.ToLower(para)
		if strings.Contains(low, "union") || strings.Contains(low, "empty") || strings.Contains(low, "no slug") {
			unionStated = true
		}
	}
	if !unionStated {
		t.Errorf("%s does not state that %d of the %d firing reports have an entirely empty SelectedSlugs union — without it the rate reads as a false-positive rate when it is mostly Gate A reporting that it checked nothing",
			rel, measuredEmptyUnion, measuredParseMissFires)
	}
}

// TestC1685_012_ExplanationDropsConditionalFraming is M1's second half: the
// Limitations section presents the every-cycle warning as a hypothetical future
// while the persona is already drifting on ~7 of every 10 cycles, and the
// one-cycle-in-seven figure is offered as the advisory's fire rate when it is
// only the marginal contribution of one formatting variant.
//
// acs-predicate: config-check
//
// Waiver rationale: as for 011 — the deliverable under test is prose. The
// assertions are negative (a specific misleading framing must be GONE) and
// conditional (a figure may stay only if it is labelled for what it is), neither
// of which a magic string can satisfy.
func TestC1685_012_ExplanationDropsConditionalFraming(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, doc := explanationDoc(t, root)

	limitations := sectionBody(doc, "## Limitations")
	if strings.TrimSpace(limitations) == "" {
		t.Fatalf("the explanation document has no '## Limitations' section — M1's correction is owed there")
	}
	if loc := regexp.MustCompile(`(?i)would\s+warn\s+every\s+cycle`).FindString(limitations); loc != "" {
		t.Errorf("Limitations still frames the every-cycle warning as a conditional future (%q) — measured 2026-09-15 the advisory already fires on %d of %d cycle-16* reports (%.1f%%), so this is the present tense, not a hypothetical",
			loc, measuredParseMissFires, measuredReportCorpus,
			100*float64(measuredParseMissFires)/float64(measuredReportCorpus))
	}
	if !strings.Contains(limitations, strconv.Itoa(measuredParseMissFires)) && !percentRE.MatchString(limitations) {
		t.Errorf("Limitations discusses the un-escalated repeat warning without stating the measured rate it is a limitation OF:\n%s", limitations)
	}

	// The marginal figure may remain, but only labelled as marginal. Left bare it
	// is the single fire-rate number in the document and reads as the advisory's.
	marginalRE := regexp.MustCompile(`(?i)one\s+cycle\s+in\s+seven|1\s+cycle\s+in\s+7|cycle\s+in\s+seven`)
	for _, para := range splitParagraphs(doc) {
		if !marginalRE.MatchString(para) {
			continue
		}
		if !strings.Contains(strings.ToLower(para), "marginal") {
			t.Errorf("this paragraph offers the backtick variant's one-in-seven figure without labelling it MARGINAL, so it still reads as the rate at which the advisory speaks (the actual rate is %.1f%%):\n%s",
				100*float64(measuredParseMissFires)/float64(measuredReportCorpus), para)
		}
	}
}

// explanationDoc resolves this cycle's build explanation document by glob rather
// than by its ULID-suffixed name, so a regenerated document does not silently
// leave these predicates reading a file that no longer exists.
func explanationDoc(t *testing.T, root string) (path, content string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1685-*.md"))
	if err != nil {
		t.Fatalf("glob explanation document: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("want exactly 1 docs/explain/builds/cycle-1685-*.md, found %d: %v", len(matches), matches)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s: %v", matches[0], err)
	}
	return matches[0], string(body)
}

// splitParagraphs splits markdown into blank-line-delimited blocks, which is the
// unit a hard-wrapped prose claim actually occupies.
func splitParagraphs(doc string) []string {
	return regexp.MustCompile(`\n\s*\n`).Split(doc, -1)
}

// sectionBody returns the text under heading up to the next "## " heading (or
// EOF) — the same bounding rule the package under test applies to its own
// sections.
func sectionBody(doc, heading string) string {
	start := strings.Index(doc, heading)
	if start < 0 {
		return ""
	}
	body := doc[start+len(heading):]
	if loc := regexp.MustCompile(`(?m)^## `).FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	return body
}

// --- audit round 2, finding H1 -----------------------------------------------
//
// The round-1 diff attached a NEUTRALITY CLAIM to the `slugLineRE` widening and
// shipped it in three places: the production comment (slugs.go), the explanation
// document (twice) and build-report.md. It said: "every one of those 12 also
// carries a '## Decision Trace' naming the same slugs, so the union
// SelectedSlugs returns is unchanged and no report becomes newly blockable."
//
// What round 1 measured was HEADING PRESENCE — all 12 do carry a
// "## Decision Trace". What it CLAIMED was union equality, which was never
// measured. Re-measured 2026-09-15 by calling the shipped SelectedSlugs on every
// `.evolve/runs/cycle-16*/scout-report.md` and comparing against the
// pre-widening pattern copied verbatim from `git show HEAD:...slugs.go`:
//
//	84  reports carry a "## Selected Tasks" section
//	12  state the slug in the backticked form (the document's own figure)
//	 6  return a DIFFERENT union — all six empty -> non-empty
//	 2  of those six become newly BLOCKABLE at Gate A / StageEnforce, the newly
//	    parsed slug having no eval file at either root evalFilePath checks:
//	      cycle-1664 -> settle-wait-stability-shortcircuit
//	      cycle-1669 -> verdict-tool-call-claudep
//
// The cause is the one the audit names: those reports DO carry a
// "## Decision Trace", but it states the selection as a "selected_tasks" string
// array rather than the decisionTrace[].finalDecision shape decisionTraceSelected
// reads, so the trace supplies nothing and the backticked bullet is the slug's
// only source.
//
// The widening is therefore a deliberate CAPABILITY INCREASE — Gate A now
// catching two genuinely missing evals is the gate doing its cycle-166 job — and
// NOT a neutral edit. 013/014 pin that behaviour hermetically so a later "fix"
// cannot quietly revert the widening instead of correcting the sentence;
// 015/016/017 require the repo to STATE the measured effect and to make the
// statement executable. build-report.md, the third site, is gitignored and lives
// outside the worktree, so it is dispositioned manual+checklist to the Auditor
// rather than pinned by a predicate reading an unreachable path.
const (
	measuredUnionDeltaReports = 6 // of measuredReportCorpus (84), all empty -> non-empty
	measuredNewlyBlockable    = 2 // of those 6, newly blockable at StageEnforce
)

// measuredNewlyBlockableEvidence is the concrete falsifying evidence: the two
// reports whose newly parsed slug has no eval file. Naming them is what makes
// the corrected claim re-derivable by the next reader instead of re-asserted.
var measuredNewlyBlockableEvidence = map[string]string{
	"cycle-1664": "settle-wait-stability-shortcircuit",
	"cycle-1669": "verdict-tool-call-claudep",
}

// preWideningSlugLineRE is slugLineRE exactly as it stood at HEAD before this
// cycle's widening (`git show HEAD:go/internal/evalgate/slugs.go`, line 96). It
// is the counterfactual arm: the production parser cannot be swapped back at
// run time, so proving a behavioural DELTA hermetically needs a local copy of
// what the old pattern matched.
var preWideningSlugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*([a-z0-9][a-z0-9-]*)`)

// cycle1664Report and cycle1669Report reproduce the two REAL reports that
// falsify the neutrality claim, each keeping the two properties that matter: the
// slug appears ONLY as a backticked "- **Slug:**" bullet, and the
// "## Decision Trace" is present but states its selection as a "selected_tasks"
// array, which decisionTraceSelected does not read.
const cycle1664Report = "# Scout Report — Cycle 1664\n\n" +
	"## Selected Tasks\n\n" +
	"### Task 1: settle-wait stability short-circuit\n" +
	"- **Deliverable kind:** code\n" +
	"- **Slug:** `settle-wait-stability-shortcircuit`\n" +
	"- **Complexity:** S\n\n" +
	"## Decision Trace\n\n```json\n{\n" +
	"  \"fleet_scope\": [\"nonconforming-deliverable-settle-wait-latency\"],\n" +
	"  \"selected_tasks\": [\"settle-wait-stability-shortcircuit\"]\n" +
	"}\n```\n"

const cycle1669Report = "# Scout Report — Cycle 1669\n\n" +
	"## Selected Tasks\n\n" +
	"### Task 1: verdict-as-tool-call for the headless claude -p driver\n" +
	"- **Deliverable kind:** code\n" +
	"- **Slug:** `verdict-tool-call-claudep`\n" +
	"- **Complexity:** M\n\n" +
	"## Decision Trace\n\n```json\n{\n" +
	"  \"cycle\": 1669,\n" +
	"  \"selected_tasks\": [\"verdict-tool-call-claudep\"],\n" +
	"  \"deferred\": [\"extend-structured-verdict-to-codex-agy\"]\n" +
	"}\n```\n"

// preWideningUnion returns the slugs the PRE-widening parser would have found.
// Both fixtures carry exactly one "- **Slug:**" bullet and it sits inside the
// Selected Tasks section, so matching over the whole report is equivalent to
// production's bounded match here — 013 asserts that equivalence by requiring
// the shipped parser to return exactly the one expected slug.
func preWideningUnion(report string) []string {
	var out []string
	for _, m := range preWideningSlugLineRE.FindAllStringSubmatch(report, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

// TestC1685_013_BacktickWideningIsNotUnionNeutral falsifies the shipped claim
// behaviourally and hermetically: on both real reports the trace supplies
// NOTHING, the pre-widening pattern matches NOTHING, and the shipped
// SelectedSlugs returns the slug — so the union it returns is demonstrably NOT
// unchanged by the widening.
//
// It is expected to be pre-existing GREEN: the defect H1 names is the false
// SENTENCE, not the behaviour. Its job is to make the corrected sentence
// durable — a later "repair" that reverts the widening to make the neutrality
// claim true again fails here, and should, because reverting would restore a
// spurious parse-miss WARN on 12 of 84 reports and re-hide the two genuinely
// missing evals Gate A now catches.
func TestC1685_013_BacktickWideningIsNotUnionNeutral(t *testing.T) {
	for _, tc := range []struct{ cycle, slug, report string }{
		{"cycle-1664", measuredNewlyBlockableEvidence["cycle-1664"], cycle1664Report},
		{"cycle-1669", measuredNewlyBlockableEvidence["cycle-1669"], cycle1669Report},
	} {
		t.Run(tc.cycle, func(t *testing.T) {
			// Premise 1: the report carries the "## Decision Trace" whose mere
			// presence was round 1's stated evidence for neutrality.
			if !strings.Contains(tc.report, "## Decision Trace") {
				t.Fatalf("fixture premise broken: %s no longer carries a \"## Decision Trace\" — the claim under test was ABOUT reports that carry one", tc.cycle)
			}
			// Premise 2: the pre-widening pattern found nothing here, so this
			// report really is one the widening changed.
			if before := preWideningUnion(tc.report); len(before) != 0 {
				t.Fatalf("fixture premise broken: the pre-widening pattern already matched %v in %s, so this report cannot demonstrate a widening delta", before, tc.cycle)
			}
			got := evalgate.SelectedSlugs(tc.report)
			if len(got) != 1 || got[0] != tc.slug {
				t.Fatalf("SelectedSlugs(%s fixture)=%v, want exactly [%s].\n"+
					"The widening must keep parsing the backticked bullet: the trace states its selection as a\n"+
					"\"selected_tasks\" array that decisionTraceSelected does not read, so the bullet is the ONLY\n"+
					"source of this slug. Reverting the widening to make the round-1 neutrality claim true is not\n"+
					"the repair H1 asks for — correcting the claim is.", tc.cycle, got, tc.slug)
			}
		})
	}
}

// TestC1685_014_BacktickedSlugWithNoEvalNewlyBlocksGateA is the caller proof for
// the half of the claim that says "no report becomes newly blockable". It drives
// the production reviewer (evalgate.NewReviewer, the core.WithReviewer seam) at
// StageEnforce over an A/B pair that differs ONLY by the backticked slug bullet:
// with the bullet Gate A BLOCKS, without it Gate A fail-opens. That delta is
// precisely "newly blockable", proven without touching the gitignored corpus.
//
// Also expected pre-existing GREEN, and kept for the same reason as 013.
func TestC1685_014_BacktickedSlugWithNoEvalNewlyBlocksGateA(t *testing.T) {
	review := func(t *testing.T, report string) core.ReviewResult {
		t.Helper()
		root, ws := t.TempDir(), t.TempDir()
		writeScoutReport(t, ws, report)
		return evalgate.NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
			Phase: string(core.PhaseScout), ProjectRoot: root, Workspace: ws,
		})
	}

	for cycle, slug := range measuredNewlyBlockableEvidence {
		report := cycle1664Report
		if cycle == "cycle-1669" {
			report = cycle1669Report
		}
		t.Run(cycle, func(t *testing.T) {
			// B arm: the same report with the slug bullet removed — what the
			// pre-widening parser effectively saw. Gate A had nothing to check.
			without := regexp.MustCompile("(?m)^- \\*\\*Slug:\\*\\*.*\n").ReplaceAllString(report, "")
			if strings.Contains(without, "**Slug:**") {
				t.Fatalf("counterfactual arm still carries a Slug bullet — the A/B pair is not isolated to the widening")
			}
			if got := evalgate.SelectedSlugs(without); len(got) != 0 {
				t.Fatalf("counterfactual premise broken: SelectedSlugs=%v with the bullet removed, so something other than the bullet supplies the slug", got)
			}
			if res := review(t, without); !res.Approve {
				t.Fatalf("Gate A BLOCKED the counterfactual arm (reason=%q) — without a parsed slug it must fail open, so this pair cannot show a *newly* blockable report", res.Reason)
			}

			// A arm: the real report. The widening parses the slug, it has no
			// eval file, and Gate A blocks where it previously fail-opened.
			res := review(t, report)
			if res.Approve {
				t.Fatalf("Gate A APPROVED %s, whose backticked slug %q has no eval file — then the widening really would be blocking-neutral and H1's measurement would be wrong; re-measure before editing any claim", cycle, slug)
			}
			if !strings.Contains(res.Reason, slug) {
				t.Errorf("Gate A's rejection for %s does not name %q, so the new block is not actionable; reason=%q", cycle, slug, res.Reason)
			}
		})
	}
}

// falsifiedNeutralityClaims are the assertions H1 proved false. Each must be
// GONE from every deliverable that carried it. They are matched as assertions,
// not as topics: "is NOT blocking-neutral" and "was not measured blocking-
// neutral" do not match, so the corrected sentence is free to use the same
// vocabulary as the sentence it replaces.
var falsifiedNeutralityClaims = []struct {
	what string
	re   *regexp.Regexp
}{
	{"an unqualified \"is blocking-neutral\" assertion", regexp.MustCompile(`(?i)\bis\s+(?:also\s+)?blocking-neutral\b`)},
	{"a \"was measured blocking-neutral\" assertion", regexp.MustCompile(`(?i)\b(?:was|were)\s+measured\s+blocking-neutral\b`)},
	{"the claim that the union is unchanged", regexp.MustCompile(`(?i)union[^.]{0,80}?\bis\s+unchanged\b`)},
	{"the claim that no report becomes newly blockable", regexp.MustCompile(`(?i)\bno\s+report\s+becomes\s+newly\s+blockable\b`)},
	{"the claim that no gate's blocking behaviour changes", regexp.MustCompile(`(?i)\bno\s+gate'?s?\s+blocking\s+behaviou?r\s+changes\b`)},
}

// correctedClaimGuidance is the exact remedy, quoted in every failure message so
// the rebuild is not left guessing at the shape that satisfies these predicates.
const correctedClaimGuidance = "" +
	"H1 remedy — replace the falsified neutrality claim with the MEASURED effect. Measured 2026-09-15 by\n" +
	"calling the shipped SelectedSlugs on every .evolve/runs/cycle-16*/scout-report.md and comparing it\n" +
	"against the pre-widening pattern: of the 84 reports carrying a \"## Selected Tasks\" section, 6 return a\n" +
	"DIFFERENT union (all six empty -> non-empty, because their \"## Decision Trace\" states the selection as a\n" +
	"\"selected_tasks\" array decisionTraceSelected does not read), and 2 of those 6 become newly BLOCKABLE at\n" +
	"Gate A because the newly parsed slug has no eval file: cycle-1664 (settle-wait-stability-shortcircuit)\n" +
	"and cycle-1669 (verdict-tool-call-claudep). Say that, say it is a deliberate capability increase rather\n" +
	"than a regression (Gate A catching a genuinely missing eval is its cycle-166 job), and name the two\n" +
	"reports so the next reader can re-derive it instead of trusting it."

// assertNoFalsifiedClaim fails for each falsified assertion still present in doc.
func assertNoFalsifiedClaim(t *testing.T, label, doc string) {
	t.Helper()
	for _, c := range falsifiedNeutralityClaims {
		if m := c.re.FindString(doc); m != "" {
			t.Errorf("%s still carries %s (%q).\nH1 measured the opposite; leaving the sentence in place reships the false claim.\n%s",
				label, c.what, m, correctedClaimGuidance)
		}
	}
}

// statesFraction reports whether any blank-line-delimited block of doc states
// num/den as a fraction ("6 of 84", "6 of the 84", "6/84") while also mentioning
// every required word — so a bare digit elsewhere in the document cannot satisfy
// the check.
func statesFraction(doc string, num, den int, mustMention ...string) (string, bool) {
	for _, para := range splitParagraphs(doc) {
		low := strings.ToLower(para)
		missing := false
		for _, w := range mustMention {
			if !strings.Contains(low, strings.ToLower(w)) {
				missing = true
			}
		}
		if missing {
			continue
		}
		for _, m := range numOfDenRE.FindAllStringSubmatch(para, -1) {
			if m[1] == strconv.Itoa(num) && m[2] == strconv.Itoa(den) {
				return para, true
			}
		}
	}
	return "", false
}

// precedingComment returns the contiguous "//" comment block immediately above
// decl in src — the scope a claim about that declaration actually occupies.
func precedingComment(src, decl string) string {
	i := strings.Index(src, decl)
	if i < 0 {
		return ""
	}
	lines := strings.Split(src[:i], "\n")
	var block []string
	for j := len(lines) - 1; j >= 0; j-- {
		l := strings.TrimSpace(lines[j])
		if l == "" && len(block) == 0 {
			continue
		}
		if !strings.HasPrefix(l, "//") {
			break
		}
		block = append([]string{l}, block...)
	}
	return strings.Join(block, "\n")
}

// TestC1685_015_SlugsGoStatesTheMeasuredBlockingEffect materialises H1 at the
// first of the three sites: the production comment that originated the claim.
//
// acs-predicate: config-check
//
// Waiver rationale: H1's defect IS a sentence — a false claim shipped in a
// comment — so the text is the system under test and there is no other seam to
// drive. It is not a bare grep in either direction: the load-bearing assertions
// are NEGATIVE (a specific falsified assertion must be GONE, which no magic
// string can satisfy), and the positive half requires the measured fraction and
// the two named counterexamples, which 013/014 independently prove by running
// the parser and the production reviewer.
func TestC1685_015_SlugsGoStatesTheMeasuredBlockingEffect(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(evalgatePkgDir(root), "slugs.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := string(body)

	assertNoFalsifiedClaim(t, "go/internal/evalgate/slugs.go", src)

	claim := precedingComment(src, "var slugLineRE = ")
	if strings.TrimSpace(claim) == "" {
		t.Fatalf("slugLineRE carries no doc comment at all — the widening must still explain itself, now with the measured effect.\n%s", correctedClaimGuidance)
	}
	if _, ok := statesFraction(claim, measuredUnionDeltaReports, measuredReportCorpus, "union"); !ok {
		t.Errorf("slugLineRE's comment does not state how many reports the widening actually changes the union for "+
			"(want %d of %d, in a sentence that says \"union\").\nIt is the sentence that made the false claim, so it is where the measured one belongs.\ncomment:\n%s\n\n%s",
			measuredUnionDeltaReports, measuredReportCorpus, claim, correctedClaimGuidance)
	}
	if !regexp.MustCompile(`(?i)\b`+strconv.Itoa(measuredNewlyBlockable)+`\b[^.]{0,160}block`).MatchString(claim) &&
		!regexp.MustCompile(`(?i)block[^.]{0,160}\b`+strconv.Itoa(measuredNewlyBlockable)+`\b`).MatchString(claim) {
		t.Errorf("slugLineRE's comment does not state that %d of those reports become newly BLOCKABLE.\n"+
			"\"the union changes\" alone understates it: the union changing is harmless, the new block is the consequence H1 is about.\ncomment:\n%s\n\n%s",
			measuredNewlyBlockable, claim, correctedClaimGuidance)
	}
	for cycle, slug := range measuredNewlyBlockableEvidence {
		if !strings.Contains(claim, cycle) && !strings.Contains(claim, slug) {
			t.Errorf("slugLineRE's comment names neither %s nor its slug %q — the two newly blockable reports ARE the falsifying evidence, and an unnamed count is another claim to be trusted rather than re-derived.\n%s",
				cycle, slug, correctedClaimGuidance)
		}
	}
	if !isoDateRE.MatchString(claim) {
		t.Errorf("slugLineRE's comment carries no YYYY-MM-DD measurement date for the corrected figures (the file already uses this convention):\n%s", claim)
	}
}

// TestC1685_016_ExplanationStatesTheMeasuredBlockingEffect materialises H1 at
// the second site. The document carries the claim TWICE — once in the body
// paragraph about the widening and once in "## Compatibility" — and both must go.
//
// acs-predicate: config-check
//
// Waiver rationale: as for 015 — the deliverable under test is prose, the
// load-bearing assertions are negative, and the positive half demands figures
// 013/014 prove behaviourally.
func TestC1685_016_ExplanationStatesTheMeasuredBlockingEffect(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, doc := explanationDoc(t, root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relativise %s: %v", path, err)
	}

	assertNoFalsifiedClaim(t, rel, doc)

	stated, ok := statesFraction(doc, measuredUnionDeltaReports, measuredReportCorpus, "union")
	if !ok {
		t.Errorf("%s states the widening's union delta NOWHERE: no paragraph gives %d of %d while mentioning the union.\n%s",
			rel, measuredUnionDeltaReports, measuredReportCorpus, correctedClaimGuidance)
	}
	if ok && !regexp.MustCompile(`(?i)\b`+strconv.Itoa(measuredNewlyBlockable)+`\b`).MatchString(stated) {
		t.Errorf("the union-delta paragraph of %s does not carry the count of newly blockable reports (%d) — the delta without its consequence is the same half-measurement H1 is about:\n%s",
			rel, measuredNewlyBlockable, stated)
	}
	for cycle, slug := range measuredNewlyBlockableEvidence {
		if !strings.Contains(doc, cycle) && !strings.Contains(doc, slug) {
			t.Errorf("%s names neither %s nor %q — the document must name the two reports that falsified its own claim, or the corrected figure is just another unverifiable assertion.\n%s",
				rel, cycle, slug, correctedClaimGuidance)
		}
	}

	compat := sectionBody(doc, "## Compatibility")
	if strings.TrimSpace(compat) == "" {
		t.Fatalf("%s has no '## Compatibility' section — it is the second site that carried the falsified claim", rel)
	}
	if !regexp.MustCompile(`(?i)newly[^.]{0,80}block|block[^.]{0,80}newly`).MatchString(compat) {
		t.Errorf("'## Compatibility' still reads as a no-behaviour-change section: it never says that some reports become newly blockable.\n"+
			"That is the one compatibility fact this diff actually has, and it is the fact the round-1 text denied.\nsection:\n%s\n\n%s", compat, correctedClaimGuidance)
	}
}

// TestC1685_017_WideningEffectIsPinnedByATrackedInPackageTest is the durable
// half of H1's remedy, and the lesson under it: the round-1 claim was never
// RUN. A corrected sentence that is still only a sentence is the same artifact
// one measurement later — the next reader has no way to re-derive it, and the
// gitignored corpus it was measured over is not in the repo.
//
// So the corrected claim must be carried by a git-TRACKED test inside the
// package it is a claim about, which asserts the widening's effect hermetically
// and passes. The `--- PASS:` line is required because `go test -run <name>`
// whose pattern matches NOTHING still exits 0 — the vacuous-pass hole.
func TestC1685_017_WideningEffectIsPinnedByATrackedInPackageTest(t *testing.T) {
	const name = "TestSlugLineWideningIsNotBlockingNeutral"
	root := acsassert.RepoRoot(t)

	carriers := sourcesNaming(t, evalgatePkgDir(root), "_test.go", "func "+name+"(")
	if len(carriers) == 0 {
		t.Fatalf("no _test.go in internal/evalgate declares func %s.\n"+
			"H1's root cause is that the neutrality claim was asserted, never executed. Add a test of that name\n"+
			"pinning the measured effect hermetically — a report whose slug appears ONLY as a backticked\n"+
			"\"- **Slug:**\" bullet and whose \"## Decision Trace\" uses the \"selected_tasks\" array shape must\n"+
			"yield that slug from SelectedSlugs, and Gate A must block it when no eval file exists. Use the real\n"+
			"cycle-1664 and cycle-1669 shapes; do NOT read .evolve/runs (gitignored, and absent in CI).\n\n%s",
			name, correctedClaimGuidance)
	}
	for _, f := range carriers {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatalf("relativise %s: %v", f, err)
		}
		if _, _, code := runIn(t, root, "git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is UNTRACKED — a gitignored or unadded test file is dropped at ship, so the executable form of the corrected claim would not survive the cycle", rel)
		}
	}

	stdout, stderr, code := runIn(t, filepath.Join(root, "go"), "go", "test", "-count=1", "-v", "-run", "^"+name+"$", "./internal/evalgate")
	if code != 0 {
		t.Fatalf("go test -run ^%s$ ./internal/evalgate exited %d\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s never RAN (exit 0 with no '--- PASS: %s' line) — a -run pattern matching no test passes vacuously, which is exactly the shape of unverified claim H1 is about", name, name)
	}

	var pinned int
	for _, src := range packageSources(t, evalgatePkgDir(root)) {
		for _, slug := range measuredNewlyBlockableEvidence {
			if strings.Contains(src, slug) {
				pinned++
			}
		}
	}
	if pinned < len(measuredNewlyBlockableEvidence) {
		t.Errorf("internal/evalgate does not pin both falsifying slugs (%q, %q) — the test must reproduce the REAL reports that disproved the claim, not a synthetic stand-in that can drift from them",
			measuredNewlyBlockableEvidence["cycle-1664"], measuredNewlyBlockableEvidence["cycle-1669"])
	}
}
