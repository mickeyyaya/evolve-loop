//go:build acs

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

const cycle1570Report = "# Scout Report\n\n## Selected Tasks\n\n" +
	"### Task 1: Config gate default policy authority\n" +
	"Task slug: config-gate-default-policy-authority\n" +
	"- **Type:** bug\n- **Complexity:** S\n\n" +
	"No eval file was authored for this task; the slug is only named in prose above.\n"

func evalgatePkgDir(root string) string { return filepath.Join(root, "go", "internal", "evalgate") }

func TestC1685_001_ParseMissTrueOnCycle1570Shape(t *testing.T) {
	if got := evalgate.SelectedSlugs(cycle1570Report); got != nil {
		t.Fatalf("fixture premise broken: the cycle-1570 shape must still parse to ZERO slugs for this predicate to exercise the reported bug; SelectedSlugs()=%v", got)
	}
	if !evalgate.SelectedTasksParseMiss(cycle1570Report) {
		t.Errorf("SelectedTasksParseMiss(cycle-1570 shape)=false, want true — a '## Selected Tasks' section carrying real, unparseable task prose is still indistinguishable from a genuine convergence report, which is the whole defect")
	}
}

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

const (
	measuredParseMissFires = 59
	measuredReportCorpus   = 84
	measuredEmptyUnion     = 56
)

var numOfDenRE = regexp.MustCompile(`(\d{1,4})\s*(?:/|of\s+(?:the\s+)?)\s*(\d{1,4})`)

var percentRE = regexp.MustCompile(`(\d{1,3}(?:\.\d+)?)\s*%`)

var isoDateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// acs-predicate: config-check
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

// acs-predicate: config-check
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

func splitParagraphs(doc string) []string {
	return regexp.MustCompile(`\n\s*\n`).Split(doc, -1)
}

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

const (
	measuredUnionDeltaReports = 6
	measuredNewlyBlockable    = 2
)

var measuredNewlyBlockableEvidence = map[string]string{
	"cycle-1664": "settle-wait-stability-shortcircuit",
	"cycle-1669": "verdict-tool-call-claudep",
}

var preWideningSlugLineRE = regexp.MustCompile(`(?m)^[*\-]\s*\*\*Slug:\*\*\s*([a-z0-9][a-z0-9-]*)`)

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

func preWideningUnion(report string) []string {
	var out []string
	for _, m := range preWideningSlugLineRE.FindAllStringSubmatch(report, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestC1685_013_BacktickWideningIsNotUnionNeutral(t *testing.T) {
	for _, tc := range []struct{ cycle, slug, report string }{
		{"cycle-1664", measuredNewlyBlockableEvidence["cycle-1664"], cycle1664Report},
		{"cycle-1669", measuredNewlyBlockableEvidence["cycle-1669"], cycle1669Report},
	} {
		t.Run(tc.cycle, func(t *testing.T) {
			if !strings.Contains(tc.report, "## Decision Trace") {
				t.Fatalf("fixture premise broken: %s no longer carries a \"## Decision Trace\" — the claim under test was ABOUT reports that carry one", tc.cycle)
			}
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

func assertNoFalsifiedClaim(t *testing.T, label, doc string) {
	t.Helper()
	for _, c := range falsifiedNeutralityClaims {
		if m := c.re.FindString(doc); m != "" {
			t.Errorf("%s still carries %s (%q).\nH1 measured the opposite; leaving the sentence in place reships the false claim.\n%s",
				label, c.what, m, correctedClaimGuidance)
		}
	}
}

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

// acs-predicate: config-check
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

// acs-predicate: config-check
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
