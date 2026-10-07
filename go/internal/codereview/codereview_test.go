package codereview

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const twoFindings = "## Findings\n\n" +
	"### CR1 (HIGH) — the grant ignores rounds already spent\n" +
	"- Dimension: correctness\n" +
	"- Location: go/internal/core/findings_repair.go:42\n" +
	"- Scenario: a second FAIL grants a third dispatch\n" +
	"- Evidence: mutant >= to > survives TestGrant_NoSecondRound\n" +
	"- Fix: compare attempts >= rounds; add TestGrant_RoundsSpentDenies\n\n" +
	"```\n### CR9 (CRITICAL) — a fenced example is never a finding\n```\n\n" +
	"### CR2 (LOW) — a name hides the concept\n" +
	"- Dimension: maintainability\n" +
	"- Location: go/internal/core/x.go:7\n" +
	"- Scenario: readers mistake d for a duration\n" +
	"- Evidence: traced lines 7-9\n\n"

func plan(na ...string) string {
	var b strings.Builder
	b.WriteString("## Review Plan\n- Inputs read: build-report.md, test-report.md, the diff\n")
	for _, k := range qualityindex.Keys() {
		if slices.Contains(na, k) {
			b.WriteString("- " + k + ": N/A — the diff never reaches " + k + "\n")
			continue
		}
		b.WriteString("- " + k + ": required (standard) — the diff touches " + k + "\n")
	}
	return b.String()
}

func scores(override map[string]string) string {
	var b strings.Builder
	b.WriteString("## Scores\n")
	for _, k := range qualityindex.Keys() {
		if v, ok := override[k]; ok {
			b.WriteString("- " + k + ": " + v + "\n")
			continue
		}
		b.WriteString("- " + k + ": 4 — the bar holds for " + k + "\n")
	}
	return b.String()
}

func reportWith(planSection, findings, scoreSection string) string {
	return planSection + "\n" + findings + "## Architecture Review\n### H1 (HIGH) — outside Findings and never parsed\n\n" + scoreSection + "\n## Verdict\nFAIL\n"
}

var twoFindingReport = reportWith(plan("concurrency"), twoFindings, scores(map[string]string{"concurrency": "N/A — the diff never reaches concurrency"}))

func TestParse_ReadsEachFindingBlockUnderFindingsOnly(t *testing.T) {
	got := Parse(strings.Replace(twoFindingReport, "### CR2", "### Notes on scope\n- Dimension: none\n\n### CR2", 1))
	want := []Finding{
		{ID: "CR1", Severity: "HIGH", Title: "the grant ignores rounds already spent", Dimension: "correctness", Location: "go/internal/core/findings_repair.go:42",
			Scenario: "a second FAIL grants a third dispatch", Evidence: "mutant >= to > survives TestGrant_NoSecondRound", Fix: "compare attempts >= rounds; add TestGrant_RoundsSpentDenies"},
		{ID: "CR2", Severity: "LOW", Title: "a name hides the concept", Dimension: "maintainability", Location: "go/internal/core/x.go:7",
			Scenario: "readers mistake d for a duration", Evidence: "traced lines 7-9"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse =\n%+v\nwant\n%+v", got, want)
	}
}

func TestParse_NoFindingsSectionOrNoneYieldsNothing(t *testing.T) {
	for _, report := range []string{"", "## Review Plan\nx\n## Verdict\nPASS\n", "## Findings\nNone.\n## Verdict\nPASS\n"} {
		if got := Parse(report); len(got) != 0 {
			t.Errorf("Parse(%q) = %+v, want no findings", report, got)
		}
	}
}

func TestParse_AMalformedFieldListKeepsTheFindingWithItsRawBody(t *testing.T) {
	report := "## Findings\n### CR1 (HIGH) — two fixes\n- Fix: one\n- Fix: two\n## Verdict\nFAIL\n"
	got := Parse(report)
	if len(got) != 1 || got[0].Severity != "HIGH" || got[0].Fix != "" || !strings.Contains(got[0].Scenario, "- Fix: one") {
		t.Fatalf("Parse = %+v, want the finding kept with its raw body as the scenario: a defect is never dropped for its format", got)
	}
}

func TestVerdict_IsDecidedByTheMostSevereFindingAgainstTheThreshold(t *testing.T) {
	high, medium, low := Finding{Severity: "HIGH"}, Finding{Severity: "MEDIUM"}, Finding{Severity: "LOW"}
	cases := []struct {
		name      string
		findings  []Finding
		threshold string
		want      string
	}{
		{"none", nil, "MEDIUM", "PASS"},
		{"below", []Finding{low}, "MEDIUM", "WARN"},
		{"at", []Finding{low, medium}, "MEDIUM", "FAIL"},
		{"above", []Finding{high}, "MEDIUM", "FAIL"},
		{"raised threshold", []Finding{medium}, "HIGH", "WARN"},
	}
	for _, tc := range cases {
		if got := Verdict(tc.findings, tc.threshold); got != tc.want {
			t.Errorf("%s: Verdict = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestRows_AreShadowDeferredWithProvenanceAndAPositionFreeText(t *testing.T) {
	rows := Rows(Parse(twoFindingReport), 2)
	if len(rows) != 2 {
		t.Fatalf("Rows = %+v, want one row per finding", rows)
	}
	first := rows[0]
	if first.Status != defectledger.StatusDeferred || first.Reason != ShadowReason || first.Source != PhaseName || first.Round != 2 || first.Severity != "HIGH" || first.Dimension != "correctness" {
		t.Errorf("row = %+v, want DEFERRED with the shadow reason, source %q, round 2, severity HIGH, dimension correctness", first, PhaseName)
	}
	wantText := "[HIGH] correctness go/internal/core/findings_repair.go:42 — the grant ignores rounds already spent | scenario: a second FAIL grants a third dispatch | evidence: mutant >= to > survives TestGrant_NoSecondRound | fix: compare attempts >= rounds; add TestGrant_RoundsSpentDenies"
	if first.Text != wantText {
		t.Errorf("text = %q\nwant %q", first.Text, wantText)
	}
	if strings.Contains(first.Text, "CR1") {
		t.Errorf("text %q carries the positional id: a renumbered re-report would mint a second row", first.Text)
	}
	if !strings.HasSuffix(rows[1].Text, "| fix: (missing)") {
		t.Errorf("text = %q, want a missing fix recorded visibly", rows[1].Text)
	}
}

func writeReport(t *testing.T, ws, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, phasecontract.ArtifactFilename(PhaseName)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func compiledSettings() Settings {
	wf := policy.Policy{}.WorkflowConfig()
	return Settings{Repair: wf.CodeReviewRepair, Index: wf.QualityIndex}
}

func readLedger(t *testing.T, ws string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, defectledger.LedgerFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRecord_AppendsTheFindingsToTheWorkspaceLedgerBesideTheAuditsRows(t *testing.T) {
	ws := t.TempDir()
	if err := defectledger.Write(ws, defectledger.Doc{OriginCycle: 11, Entries: []defectledger.Entry{{ID: defectledger.ID("audit row"), Text: "audit row", Status: defectledger.StatusOpen}}}); err != nil {
		t.Fatal(err)
	}
	writeReport(t, ws, twoFindingReport)

	out := Record(Request{Workspace: ws, Cycle: 12, Round: 1}, compiledSettings())

	if out.Err != nil || len(out.Findings) != 2 {
		t.Fatalf("Record = %+v, want two findings recorded", out)
	}
	doc, _, err := defectledger.Read(ws)
	if err != nil || doc.OriginCycle != 11 || len(doc.Entries) != 3 || doc.Entries[0].Text != "audit row" {
		t.Fatalf("ledger = %+v (err %v), want the audit row kept first and origin 11", doc, err)
	}
	before := readLedger(t, ws)
	if again := Record(Request{Workspace: ws, Cycle: 12, Round: 1}, compiledSettings()); again.Err != nil || readLedger(t, ws) != before {
		t.Errorf("a repeated record changed the ledger (err %v): the rows merge by text", again.Err)
	}
}

func TestRecord_FirstWriterStampsTheOrigin(t *testing.T) {
	ws := t.TempDir()
	writeReport(t, ws, twoFindingReport)
	if out := Record(Request{Workspace: ws, Cycle: 31, Round: 1}, compiledSettings()); out.Err != nil {
		t.Fatal(out.Err)
	}
	doc, _, _ := defectledger.Read(ws)
	if doc.OriginCycle != 31 {
		t.Errorf("origin = %d, want 31", doc.OriginCycle)
	}
}

func TestRecord_NoFindingsWritesNoLedger(t *testing.T) {
	ws := t.TempDir()
	writeReport(t, ws, reportWith(plan(), "## Findings\nNone.\n", scores(nil)))
	out := Record(Request{Workspace: ws, Cycle: 5, Round: 1}, compiledSettings())
	if out.Err != nil || len(out.Findings) != 0 {
		t.Fatalf("Record = %+v, want a clean review with no findings", out)
	}
	if _, err := os.Stat(filepath.Join(ws, defectledger.LedgerFile)); !os.IsNotExist(err) {
		t.Errorf("a clean review minted a ledger (stat err %v): every later reader would see a defect file with no defect", err)
	}
}

func TestRecord_FaultsAreReportedNeverRaised(t *testing.T) {
	missing := Record(Request{Workspace: t.TempDir(), Cycle: 5, Round: 1}, compiledSettings())
	if missing.Err == nil || len(missing.Findings) != 0 {
		t.Errorf("an absent report = %+v, want an error and nothing recorded", missing)
	}
	ws := t.TempDir()
	writeReport(t, ws, twoFindingReport)
	if err := os.Mkdir(filepath.Join(ws, defectledger.LedgerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if out := Record(Request{Workspace: ws, Cycle: 5, Round: 1}, compiledSettings()); out.Err == nil || !strings.Contains(out.Err.Error(), "read the defect ledger") {
		t.Errorf("an unreadable ledger = %+v, want the read error reported", out)
	}
	ro := t.TempDir()
	writeReport(t, ro, twoFindingReport)
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	if out := Record(Request{Workspace: ro, Cycle: 5, Round: 1}, compiledSettings()); out.Err == nil {
		t.Errorf("a read-only workspace = %+v, want the write error reported", out)
	}
}

func TestEvent_CountsBySeverityAndCarriesTheScoresVector(t *testing.T) {
	ws := t.TempDir()
	writeReport(t, ws, twoFindingReport)
	e := Record(Request{Workspace: ws, Cycle: 12, Round: 1}, compiledSettings()).Event()

	if e.Code != CodeFindings || e.Module != signalcenter.ModuleReview || e.Kind != signalcenter.KindPhaseOutcome || e.Severity != signalcenter.SeverityInfo || e.Cycle != 12 || e.Phase != PhaseName {
		t.Fatalf("event = %+v, want an INFO REVIEW_FINDINGS phase outcome for cycle 12", e)
	}
	want := map[string]string{"findings": "critical=0,high=1,medium=0,low=1", "verdict": "FAIL", "would_repair": "true",
		"stage": "shadow", "threshold": "MEDIUM", "round": "1", "recorded": "true",
		"scores": "correctness=4,architecture=4,maintainability=4,test-quality=4,robustness=4,concurrency=N/A,performance=4,debuggability=4,security=4,docs-consistency=4",
		"gaps":   ""}
	if !reflect.DeepEqual(e.Fields, want) {
		t.Errorf("fields = %v\nwant     %v", e.Fields, want)
	}
	if !strings.Contains(e.Reason, "2 finding(s)") || !strings.Contains(e.Reason, "FAIL") {
		t.Errorf("reason %q, want the count and the verdict", e.Reason)
	}
}

func TestEvent_WouldRepairIsAnyFindingOrAScoresGap(t *testing.T) {
	cases := map[string]struct {
		findings, scoreSection string
		want, gaps             string
	}{
		"clean":          {"## Findings\nNone.\n", scores(nil), "false", ""},
		"a LOW finding":  {"## Findings\n### CR1 (LOW) — nit\n- Dimension: maintainability\n- Fix: rename\n", scores(nil), "true", ""},
		"a gap, no rows": {"## Findings\nNone.\n", scores(map[string]string{"performance": "3 — CR7: slow path"}), "true", "performance=3<4"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws := t.TempDir()
			writeReport(t, ws, reportWith(plan(), tc.findings, tc.scoreSection))
			e := Record(Request{Workspace: ws, Cycle: 2, Round: 1}, compiledSettings()).Event()
			if e.Fields["would_repair"] != tc.want || e.Fields["gaps"] != tc.gaps {
				t.Errorf("would_repair=%s gaps=%q, want %s %q: round 1 resolves only with no finding and qualifying scores", e.Fields["would_repair"], e.Fields["gaps"], tc.want, tc.gaps)
			}
		})
	}
}

func TestEvent_WarnsOnARecordingFaultOrAConfigWarning(t *testing.T) {
	failed := Record(Request{Workspace: t.TempDir(), Cycle: 3, Round: 1}, compiledSettings()).Event()
	if failed.Severity != signalcenter.SeverityWarn || failed.Fields["recorded"] != "false" || failed.Fields["error"] == "" {
		t.Errorf("fault event = %+v, want WARN, recorded=false and the error", failed)
	}
	ws := t.TempDir()
	writeReport(t, ws, reportWith(plan(), "## Findings\nNone.\n", scores(nil)))
	cfg := compiledSettings()
	cfg.Repair.Warnings = []string{"workflow.findings_repair.code-review.stage: \"enforce\" lands with the loop"}
	cfg.Index.Warnings = []string{"workflow.quality_index.thresholds: unknown dimension \"speed\" ignored"}
	warned := Record(Request{Workspace: ws, Cycle: 3, Round: 1}, cfg).Event()
	if warned.Severity != signalcenter.SeverityWarn || warned.Fields["config_warning"] != cfg.Repair.Warnings[0]+"; "+cfg.Index.Warnings[0] || warned.Fields["verdict"] != "PASS" || warned.Fields["recorded"] != "true" {
		t.Errorf("config-warning event = %+v, want WARN carrying both blocks' warnings on a clean PASS", warned)
	}
}

func TestOutcome_EventProjectsAnOutcomeBuiltDirectly(t *testing.T) {
	o := Outcome{
		Request:  Request{Workspace: "/unused", Cycle: 9, Round: 2},
		Settings: compiledSettings(),
		Findings: []Finding{{Severity: "CRITICAL"}, {Severity: "MEDIUM"}},
		Overflow: 3,
		Err:      errors.New("write the defect ledger: disk full"),
	}
	e := o.Event()
	if e.Fields["findings"] != "critical=1,high=0,medium=1,low=0" || e.Fields["overflow"] != "3" || e.Fields["recorded"] != "false" ||
		e.Fields["scores"] != "" || e.Severity != signalcenter.SeverityWarn || e.Cycle != 9 {

		t.Errorf("event = %+v, want the counts, the overflow, an unrecorded WARN, and no scores: a missing vector never qualifies", e)
	}
	if !strings.Contains(e.Fields["gaps"], "correctness=missing") {
		t.Errorf("gaps = %q, want every dimension missing", e.Fields["gaps"])
	}
}

func TestParse_AFindingsDimensionIsReadInTheIndexsLowerCase(t *testing.T) {
	findings := "## Findings\n### CR1 (LOW) — a nit on the hot path\n- Dimension: Performance\n- Location: x.go:1\n- Scenario: s\n- Evidence: e\n- Fix: f\n"
	report := reportWith(plan(), findings, scores(map[string]string{"performance": "3 — CR1"}))
	thresholds, _ := qualityindex.ResolveThresholds(nil)

	got := Parse(report)

	if len(got) != 1 || got[0].Dimension != "performance" || Rows(got, 1)[0].Dimension != "performance" {
		t.Fatalf("Parse = %+v, want the dimension performance, the index's own spelling, on the finding and its row", got)
	}
	if problems := ValidateReport(report, thresholds); len(problems) != 0 {
		t.Errorf("problems = %q, want none: the gap on performance cites CR1, a performance finding", problems)
	}
}

func TestValidateReport_AWellFormedReportHasNoProblems(t *testing.T) {
	gapped := reportWith(plan("concurrency"), twoFindings, scores(map[string]string{
		"concurrency": "N/A — no goroutine", "correctness": "3 — CR1: the grant ignores spent rounds",
	}))
	thresholds, _ := qualityindex.ResolveThresholds(nil)
	if problems := ValidateReport(gapped, thresholds); len(problems) != 0 {
		t.Errorf("problems = %q, want none: the gap cites CR1, a correctness finding", problems)
	}
	for _, none := range []string{"None.", "none"} {
		if problems := ValidateReport(reportWith(plan(), "## Findings\n"+none+"\n", scores(nil)), thresholds); len(problems) != 0 {
			t.Errorf("Findings %q: problems = %q, want none: a clean review writes None.", none, problems)
		}
	}
}

func TestValidateReport_EveryGrammarBreachIsAProblem(t *testing.T) {
	thresholds, _ := qualityindex.ResolveThresholds(nil)
	cases := map[string]struct {
		report string
		want   string
	}{
		"no plan":                  {strings.Replace(twoFindingReport, "## Review Plan", "## Plan", 1), "## Review Plan"},
		"no scores":                {strings.Replace(twoFindingReport, "## Scores", "## Grades", 1), "## Scores"},
		"plan and scores disagree": {reportWith(plan(), twoFindings, scores(map[string]string{"performance": "N/A — no path"})), "performance"},
		"a gap cites nothing":      {reportWith(plan(), twoFindings, scores(map[string]string{"security": "3 — weak"})), "security"},
		"a gap cites a finding on another dimension":  {reportWith(plan(), twoFindings, scores(map[string]string{"security": "2 — CR1"})), "security"},
		"a gap cites an id the report lacks":          {reportWith(plan(), twoFindings, scores(map[string]string{"maintainability": "3 — CR5"})), "maintainability"},
		"a heading severity in title case":            {reportWith(plan(), "## Findings\n### CR1 (High) — inverted guard\n- Dimension: correctness\n- Fix: f\n", scores(nil)), "### CR1 (High)"},
		"a finding heading without a severity":        {reportWith(plan(), "## Findings\n### CR1 — inverted guard\n- Dimension: correctness\n- Fix: f\n", scores(nil)), "### CR1 — inverted guard"},
		"a finding at heading level four":             {reportWith(plan(), "## Findings\n#### CR1 (HIGH) — inverted guard\n- Dimension: correctness\n", scores(nil)), "#### CR1 (HIGH)"},
		"a finding written as a bullet":               {reportWith(plan(), "## Findings\n- CR1 (HIGH) — inverted guard; dimension correctness\n", scores(nil)), "- CR1 (HIGH)"},
		"a heading under Findings that is no finding": {reportWith(plan(), twoFindings+"### Notes on scope\n- Dimension: none\n", scores(nil)), "### Notes on scope"},
		"a drifted finding beside a parsed one":       {reportWith(plan(), twoFindings+"#### CR3 (HIGH) — a third\n", scores(nil)), "#### CR3 (HIGH)"},
		"a numbered finding beside a parsed one":      {reportWith(plan(), twoFindings+"1. CR3 (HIGH) — a third\n", scores(nil)), "1. CR3 (HIGH)"},
		"a bold bullet finding beside a parsed one":   {reportWith(plan(), twoFindings+"- **CR3** (HIGH) — a third\n", scores(nil)), "- **CR3** (HIGH)"},
		"prose that is neither findings nor None.":    {reportWith(plan(), "## Findings\nTwo issues: the guard and the name.\n", scores(nil)), "None."},
		"no findings section":                         {strings.Replace(twoFindingReport, "## Findings", "## Notes", 1), "## Findings"},
		"a duplicated findings section":               {reportWith(plan(), twoFindings+"## Findings\nNone.\n", scores(nil)), "duplicate ## Findings"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			problems := ValidateReport(tc.report, thresholds)
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Errorf("problems = %q, want exactly one naming %q", problems, tc.want)
			}
		})
	}
}

func TestValidateReport_TheGapRuleReadsTheConfiguredThreshold(t *testing.T) {
	report := reportWith(plan(), twoFindings, scores(map[string]string{"security": "3 — weak but at the bar"}))
	lowered, _ := qualityindex.ResolveThresholds(map[string]int{"security": 3})
	if problems := ValidateReport(report, lowered); len(problems) != 0 {
		t.Errorf("a 3 against a configured threshold of 3: problems = %q, want none", problems)
	}
}

func TestSkipped_IsAWarnNamingTheReasonAndTheRoundUnderTheReviewModule(t *testing.T) {
	e := Skipped(Request{Cycle: 33, Round: 2}, SkipMalformed)

	if m, ok := signalcenter.IsRegistered(CodeSkipped); !ok || m != signalcenter.ModuleReview {
		t.Fatalf("REVIEW_SKIPPED registered=%v module=%q, want module review", ok, m)
	}
	if e.Code != CodeSkipped || e.Module != signalcenter.ModuleReview || e.Severity != signalcenter.SeverityWarn || e.Cycle != 33 || e.Phase != PhaseName {
		t.Errorf("event = %+v, want a WARN REVIEW_SKIPPED for cycle 33 under the review module", e)
	}
	if e.Fields["reason"] != "malformed" || e.Fields["round"] != "2" || !strings.Contains(e.Reason, "round 2 skipped (malformed)") {
		t.Errorf("fields = %v, reason %q, want reason malformed and round 2", e.Fields, e.Reason)
	}
}

func TestCodeFindings_IsRegisteredUnderTheReviewModule(t *testing.T) {
	if m, ok := signalcenter.IsRegistered(CodeFindings); !ok || m != signalcenter.ModuleReview {
		t.Errorf("REVIEW_FINDINGS registered=%v module=%q, want module review", ok, m)
	}
}

func TestPhaseName_IsThePhaseJSONsNameArtifactSectionsAndGrammar(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "phases", PhaseName, "phase.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Name    string `json:"name"`
		Outputs struct {
			Files []string `json:"files"`
		} `json:"outputs"`
		PromptContext []string `json:"prompt_context"`
		Classify      struct {
			RequireSections []string `json:"require_sections"`
			Grammars        []string `json:"grammars"`
		} `json:"classify"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Name != PhaseName || len(spec.Outputs.Files) != 1 || filepath.Base(spec.Outputs.Files[0]) != phasecontract.ArtifactFilename(PhaseName) {
		t.Errorf("phase.json name %q outputs %v, want %q writing %s", spec.Name, spec.Outputs.Files, PhaseName, phasecontract.ArtifactFilename(PhaseName))
	}
	for _, section := range []string{qualityindex.PlanSection, FindingsSection, qualityindex.ScoresSection} {
		if !slices.Contains(spec.Classify.RequireSections, section) {
			t.Errorf("phase.json requires %v, want %q: the parsers read it", spec.Classify.RequireSections, section)
		}
	}
	if !reflect.DeepEqual(spec.Classify.Grammars, []string{phasespec.GrammarCodeReviewReport}) {
		t.Errorf("phase.json grammars = %v, want [%s]: the gate checks the report grammar", spec.Classify.Grammars, phasespec.GrammarCodeReviewReport)
	}
	if want := []string{"goal", "task_contract"}; !reflect.DeepEqual(spec.PromptContext, want) {
		t.Errorf("phase.json prompt_context = %v, want %v: the evaluate default goal line, then the acceptance the reviewer judges against", spec.PromptContext, want)
	}
}

func TestThePersonasFindingExampleParsesWithEveryField(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "agents", "evolve-code-reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	persona := string(raw)
	start := strings.Index(persona, "### CR1 (")
	end := strings.Index(persona[max(start, 0):], "```")
	if start < 0 || end < 0 {
		t.Fatalf("the persona has no fenced CR1 example to check (start %d, end %d)", start, end)
	}
	got := Parse("## " + FindingsSection + "\n" + persona[start:start+end])
	if len(got) != 1 {
		t.Fatalf("the persona's example parses to %+v, want one finding", got)
	}
	f := got[0]
	for name, value := range map[string]string{"Dimension": f.Dimension, "Location": f.Location, "Scenario": f.Scenario, "Evidence": f.Evidence, "Fix": f.Fix} {
		if value == "" {
			t.Errorf("the persona's example leaves %s empty under the parser: the grammar it teaches and the one the kernel reads have drifted", name)
		}
	}
	if !qualityindex.Known(f.Dimension) {
		t.Errorf("the persona's example names dimension %q, which is not in the index", f.Dimension)
	}
}

func TestSeverityFields_AreTheFindingVocabularyInRankOrder(t *testing.T) {
	for i, severity := range severityFields {
		if got := reportdoc.SeverityRank(severity); got != i {
			t.Errorf("SeverityRank(%s) = %d, want %d: the signal's count fields must be reportdoc's vocabulary in its order", severity, got, i)
		}
	}
	if unknown := reportdoc.SeverityRank("NOT-A-SEVERITY"); unknown != len(severityFields) {
		t.Errorf("reportdoc ranks %d severities, the signal counts %d", unknown, len(severityFields))
	}
}

func TestRoundFrom_CountsTheCompletedCodeReviewDispatches(t *testing.T) {
	cases := map[int][]string{
		0: {"scout", "build"},
		1: {"scout", "build", PhaseName},
		2: {"scout", "build", PhaseName, "build", PhaseName, "audit"},
	}
	for want, completed := range cases {
		if got := RoundFrom(completed); got != want {
			t.Errorf("RoundFrom(%v) = %d, want %d", completed, got, want)
		}
	}
}

func TestRecord_ACutBeyondTheCapIsCountedAndNeverMintsAnOpenRow(t *testing.T) {
	ws := t.TempDir()
	full := defectledger.Doc{OriginCycle: 4}
	for i := 0; i < defectledger.MaxEntries; i++ {
		text := "earlier finding " + strconv.Itoa(i)
		full.Entries = append(full.Entries, defectledger.Entry{Text: text, Status: defectledger.StatusDeferred, Source: PhaseName})
	}
	if err := defectledger.Write(ws, full); err != nil {
		t.Fatal(err)
	}
	writeReport(t, ws, twoFindingReport)

	out := Record(Request{Workspace: ws, Cycle: 4, Round: 1}, compiledSettings())

	if out.Err != nil || out.Overflow != 2 || out.Event().Fields["overflow"] != "2" {
		t.Fatalf("Record = %+v, want both findings cut and counted on the signal", out)
	}
	doc, _, _ := defectledger.Read(ws)
	for _, e := range doc.Entries[defectledger.MaxEntries:] {
		if e.Status != defectledger.StatusDeferred || e.Source != PhaseName || e.Severity != "HIGH" {
			t.Errorf("stand-in row = %+v, want DEFERRED, sourced and at the highest cut severity: a shadow cut must never be owed", e)
		}
	}
}

func TestEvent_TheWorstCaseFieldSetPassesTheCenterUntruncated(t *testing.T) {
	cfg := compiledSettings()
	cfg.Repair.Warnings = []string{"stage warning"}
	o := Outcome{Request: Request{Cycle: 4, Round: 1}, Settings: cfg, Findings: []Finding{{Severity: "HIGH"}}, Overflow: 2, Err: errors.New("disk full")}
	center := signalcenter.New()
	var got []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { got = append(got, e) })
	center.Emit(o.Event())
	if len(got) != 1 || got[0].Fields["truncated"] != "" || got[0].Fields["would_repair"] == "" || got[0].Fields["verdict"] == "" {
		t.Fatalf("delivered %+v, want every field kept: the Center keeps at most %d, and a dropped field is a silent metric loss", got, signalcenter.MaxFields)
	}
}
