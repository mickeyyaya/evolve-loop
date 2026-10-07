package qualityindex

import (
	"reflect"
	"strings"
	"testing"
)

func fullScores(override map[string]string) string {
	var b strings.Builder
	b.WriteString("## Scores\n")
	for _, k := range Keys() {
		line := "- " + k + ": 4 — every bar met for " + k
		if v, ok := override[k]; ok {
			line = v
		}
		if line != "" {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func fullPlan(override map[string]string) string {
	var b strings.Builder
	b.WriteString("## Review Plan\n- Inputs read: build-report.md, test-report.md, the diff\n")
	for _, k := range Keys() {
		line := "- " + k + ": required (standard) — the diff touches " + k
		if v, ok := override[k]; ok {
			line = v
		}
		if line != "" {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func TestKeys_AreTheTenDimensionsInIndexOrder(t *testing.T) {
	want := []string{"correctness", "architecture", "maintainability", "test-quality", "robustness", "concurrency", "performance", "debuggability", "security", "docs-consistency"}
	if got := Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	got := Keys()
	got[0] = "mutated"
	if Keys()[0] != "correctness" {
		t.Error("Keys() returned the package's own slice: a caller mutated the vocabulary")
	}
}

func TestNeverNA_IsExactlyTheFiveDimensionsEveryCodeChangeHas(t *testing.T) {
	var never []string
	for _, k := range Keys() {
		if NeverNA(k) {
			never = append(never, k)
		}
	}
	want := []string{"correctness", "architecture", "maintainability", "test-quality", "docs-consistency"}
	if !reflect.DeepEqual(never, want) {
		t.Errorf("never-N/A = %v, want %v", never, want)
	}
	if NeverNA("not-a-dimension") || Known("not-a-dimension") || !Known("security") {
		t.Error("an unknown key must be neither known nor never-N/A, and a dimension must be known")
	}
}

func TestParseScores_ReadsEveryDimensionWithItsRationale(t *testing.T) {
	report := "intro\n" + strings.Replace(fullScores(map[string]string{
		"concurrency": "- concurrency: N/A – no goroutine, channel or lock in the diff",
		"performance": "- Performance: 3 - CR2: readAll per call at ledger.go:120",
		"security":    "- security: n/a — no input boundary in the diff",
		"robustness":  "- **robustness**: 5 — every error is wrapped",
	}), "## Scores\n", "## Scores\nScored against the default thresholds; prose lines are not entries.\n", 1) + "## Verdict\nWARN\n"
	scores, problems := ParseScores(report)
	if len(problems) != 0 {
		t.Fatalf("problems = %q, want none: en dash, hyphen, a capitalised key, a bold key and a lower-case n/a are accepted", problems)
	}
	if got := scores["security"]; !got.NA || got.Rationale != "no input boundary in the diff" {
		t.Errorf("security = %+v, want N/A in any letter case", got)
	}
	if got := scores["robustness"]; got.Value != 5 || got.Rationale != "every error is wrapped" {
		t.Errorf("robustness = %+v, want 5: a **bold** key reads as reportdoc.Fields reads it", got)
	}
	if got := scores["performance"]; got.Value != 3 || got.NA || got.Rationale != "CR2: readAll per call at ledger.go:120" {
		t.Errorf("performance = %+v, want 3 with its rationale split at the first separator", got)
	}
	if got := scores["concurrency"]; !got.NA || got.Rationale != "no goroutine, channel or lock in the diff" {
		t.Errorf("concurrency = %+v, want N/A with its reason", got)
	}
	if len(scores) != len(Keys()) {
		t.Errorf("scored %d dimensions, want %d", len(scores), len(Keys()))
	}
}

func TestParseScores_EveryMalformedFormIsAProblem(t *testing.T) {
	cases := map[string]struct {
		override map[string]string
		extra    string
		want     string
	}{
		"missing dimension":   {map[string]string{"security": ""}, "", "security"},
		"duplicate line":      {nil, "- security: 4 — again\n", "security"},
		"unknown key":         {nil, "- speed: 4 — fast\n", "speed"},
		"never-N/A as N/A":    {map[string]string{"correctness": "- correctness: N/A — nothing to prove"}, "", "correctness"},
		"empty rationale":     {map[string]string{"robustness": "- robustness: 4 — "}, "", "robustness"},
		"no separator":        {map[string]string{"robustness": "- robustness: 4"}, "", "robustness"},
		"score out of range":  {map[string]string{"debuggability": "- debuggability: 6 — very good"}, "", "debuggability"},
		"score not a number":  {map[string]string{"debuggability": "- debuggability: good — fine"}, "", "debuggability"},
		"zero is not a score": {map[string]string{"debuggability": "- debuggability: 0 — none"}, "", "debuggability"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, problems := ParseScores(fullScores(tc.override) + tc.extra)
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Errorf("problems = %q, want exactly one naming %q", problems, tc.want)
			}
		})
	}
}

func TestParseScores_AMissingSectionIsOneProblem(t *testing.T) {
	_, problems := ParseScores("## Findings\nNone.\n")
	if len(problems) != 1 || !strings.Contains(problems[0], "## Scores") {
		t.Errorf("problems = %q, want one naming the missing section", problems)
	}
}

func TestParsePlan_ReadsInputsDepthsAndNAReasons(t *testing.T) {
	report := fullPlan(map[string]string{
		"concurrency": "- concurrency: required (deep) — the diff adds a goroutine in usageprobe/evidence.go:96",
		"security":    "- security: n/a — no trust boundary in the diff",
	})
	plan, problems := ParsePlan(report)
	if len(problems) != 0 {
		t.Fatalf("problems = %q, want none", problems)
	}
	if plan.InputsRead != "build-report.md, test-report.md, the diff" {
		t.Errorf("InputsRead = %q", plan.InputsRead)
	}
	if got := plan.Lines["concurrency"]; !got.Required || got.Depth != "deep" || got.Justification != "the diff adds a goroutine in usageprobe/evidence.go:96" {
		t.Errorf("concurrency = %+v, want required at deep with its justification", got)
	}
	if got := plan.Lines["security"]; got.Required || got.Justification != "no trust boundary in the diff" {
		t.Errorf("security = %+v, want an N/A line with its reason", got)
	}
}

func TestParsePlan_EveryMalformedFormIsAProblem(t *testing.T) {
	cases := map[string]struct {
		report string
		want   string
	}{
		"no inputs line":       {strings.Replace(fullPlan(nil), "- Inputs read: build-report.md, test-report.md, the diff\n", "", 1), "Inputs read"},
		"empty inputs line":    {strings.Replace(fullPlan(nil), "build-report.md, test-report.md, the diff", "", 1), "Inputs read"},
		"unknown depth":        {fullPlan(map[string]string{"performance": "- performance: required (thorough) — hot path"}), "performance"},
		"no depth":             {fullPlan(map[string]string{"performance": "- performance: required — hot path"}), "performance"},
		"empty justification":  {fullPlan(map[string]string{"performance": "- performance: required (light) — "}), "performance"},
		"missing dimension":    {fullPlan(map[string]string{"performance": ""}), "performance"},
		"duplicate dimension":  {fullPlan(nil) + "- performance: N/A — again\n", "performance"},
		"unknown key":          {fullPlan(nil) + "- vibes: required (light) — good\n", "vibes"},
		"never-N/A as N/A":     {fullPlan(map[string]string{"test-quality": "- test-quality: N/A — no tests"}), "test-quality"},
		"missing plan section": {"## Scores\n", "## Review Plan"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, problems := ParsePlan(tc.report)
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Errorf("problems = %q, want exactly one naming %q", problems, tc.want)
			}
		})
	}
}

func TestAgree_NADimensionsMustMatchBetweenPlanAndScores(t *testing.T) {
	plan, _ := ParsePlan(fullPlan(map[string]string{"security": "- security: N/A — no boundary"}))
	matched, _ := ParseScores(fullScores(map[string]string{"security": "- security: N/A — no boundary"}))
	if problems := Agree(plan, matched); len(problems) != 0 {
		t.Errorf("Agree on matching N/A sets = %q, want none", problems)
	}
	scoredAnyway, _ := ParseScores(fullScores(nil))
	planNA, _ := ParsePlan(fullPlan(map[string]string{"performance": "- performance: N/A — no executed path"}))
	scoresNA, _ := ParseScores(fullScores(map[string]string{"concurrency": "- concurrency: N/A — no goroutine"}))
	fullyRequired, _ := ParsePlan(fullPlan(nil))
	for name, problems := range map[string][]string{
		"plan N/A, scored":   Agree(planNA, scoredAnyway),
		"plan required, N/A": Agree(fullyRequired, scoresNA),
	} {
		if len(problems) != 1 {
			t.Errorf("%s: problems = %q, want exactly one", name, problems)
		}
	}
}

func TestQualifies_EveryApplicableDimensionMustMeetItsThreshold(t *testing.T) {
	thresholds, _ := ResolveThresholds(nil)
	pass, _ := ParseScores(fullScores(map[string]string{"concurrency": "- concurrency: N/A — no goroutine"}))
	if ok, gaps := Qualifies(pass, thresholds); !ok || len(gaps) != 0 {
		t.Fatalf("all 4s with an allowed N/A: ok=%v gaps=%+v, want qualified", ok, gaps)
	}
	gapped, _ := ParseScores(fullScores(map[string]string{"performance": "- performance: 3 — CR2: slow"}))
	ok, gaps := Qualifies(gapped, thresholds)
	if ok || !reflect.DeepEqual(gaps, []Gap{{Dimension: "performance", Score: 3, Threshold: 4}}) {
		t.Errorf("a 3 against 4: ok=%v gaps=%+v, want one performance gap", ok, gaps)
	}
	missing := Scores{}
	for k, v := range pass {
		missing[k] = v
	}
	delete(missing, "security")
	missing["correctness"] = Score{NA: true, Rationale: "forced"}
	ok, gaps = Qualifies(missing, thresholds)
	want := []Gap{{Dimension: "correctness", Threshold: 4, Reason: "N/A not allowed"}, {Dimension: "security", Threshold: 4, Reason: "missing"}}
	if ok || !reflect.DeepEqual(gaps, want) {
		t.Errorf("missing and forbidden N/A: ok=%v gaps=%+v, want %+v in index order", ok, gaps, want)
	}
	lowered := Thresholds{}
	for k, v := range thresholds {
		lowered[k] = v
	}
	lowered["performance"] = 3
	if ok, _ := Qualifies(gapped, lowered); !ok {
		t.Error("a 3 against a configured threshold of 3 must qualify: the threshold is config, not the literal 4")
	}
}

func TestResolveThresholds_StarThenPerKeyWithLoudFallbacks(t *testing.T) {
	def, warnings := ResolveThresholds(nil)
	for _, k := range Keys() {
		if def[k] != DefaultThreshold {
			t.Errorf("default %s = %d, want %d", k, def[k], DefaultThreshold)
		}
	}
	if len(warnings) != 0 {
		t.Errorf("defaults warned: %q", warnings)
	}
	got, warnings := ResolveThresholds(map[string]int{"*": 3, "security": 5, "speed": 2, "performance": 7, "architecture": 0})
	if got["correctness"] != 3 || got["security"] != 5 || got["performance"] != 3 || got["architecture"] != 3 {
		t.Errorf("resolved %v, want * = 3 everywhere, security overridden to 5, the out-of-range entries ignored", got)
	}
	if len(warnings) != 3 || !strings.Contains(strings.Join(warnings, "|"), "speed") || !strings.Contains(strings.Join(warnings, "|"), "performance") || !strings.Contains(strings.Join(warnings, "|"), "architecture") {
		t.Errorf("warnings = %q, want one each for the unknown key and the two out-of-range values", warnings)
	}
	if _, warnings := ResolveThresholds(map[string]int{"*": 9}); len(warnings) != 1 {
		t.Errorf("an out-of-range * warned %q, want one warning", warnings)
	}
}

func TestParsePlan_InputsReadKeepsItsWholeListAndAppearsOnce(t *testing.T) {
	dashed := strings.Replace(fullPlan(nil), "build-report.md, test-report.md, the diff", "intent.md - when present, build-report.md — claims only", 1)
	plan, problems := ParsePlan(dashed)
	if len(problems) != 0 || plan.InputsRead != "intent.md - when present, build-report.md — claims only" {
		t.Errorf("InputsRead = %q (problems %q), want the whole list verbatim: it is a list, not a scored line", plan.InputsRead, problems)
	}
	_, problems = ParsePlan(fullPlan(nil) + "- Inputs read: again\n")
	if len(problems) != 1 || !strings.Contains(problems[0], "more than one") {
		t.Errorf("a second Inputs read line: problems = %q, want one", problems)
	}
}

func TestParse_ADuplicatedSectionIsAProblemNotAGuess(t *testing.T) {
	twice := fullScores(nil) + "## Findings\nNone.\n" + fullScores(nil)
	if _, problems := ParseScores(twice); len(problems) != 1 || !strings.Contains(problems[0], "duplicate") {
		t.Errorf("two ## Scores sections: problems = %q, want one naming the duplicate", problems)
	}
}

func TestParseScores_TheScaleEndsAndTheEarliestSeparator(t *testing.T) {
	report := fullScores(map[string]string{
		"robustness":    "- robustness: 1 — a swallowed error on the write path",
		"debuggability": "- debuggability: 5 - every failure names its input — exemplary",
	})
	scores, problems := ParseScores(report)
	if len(problems) != 0 {
		t.Fatalf("problems = %q, want 1 and 5 accepted: they are the scale's ends", problems)
	}
	if scores["robustness"].Value != 1 {
		t.Errorf("robustness = %+v, want 1", scores["robustness"])
	}
	if got := scores["debuggability"]; got.Value != 5 || got.Rationale != "every failure names its input — exemplary" {
		t.Errorf("debuggability = %+v, want 5 split at the earliest separator, the later em dash kept in the rationale", got)
	}
	if got, warnings := ResolveThresholds(map[string]int{"performance": minScore, "security": maxScore}); got["performance"] != minScore || got["security"] != maxScore || len(warnings) != 0 {
		t.Errorf("thresholds at the scale's ends = %v (warnings %q), want both accepted", got, warnings)
	}
}

func TestParsePlan_ReturnsTheWholePlanAsValues(t *testing.T) {
	report := fullPlan(map[string]string{"security": "- security: N/A — no boundary"})
	plan, problems := ParsePlan(report)
	if len(problems) != 0 {
		t.Fatalf("problems = %q", problems)
	}
	want := Plan{InputsRead: "build-report.md, test-report.md, the diff", Lines: map[string]PlanLine{}}
	for _, k := range Keys() {
		want.Lines[k] = PlanLine{Required: true, Depth: "standard", Justification: "the diff touches " + k}
	}
	want.Lines["security"] = PlanLine{Justification: "no boundary"}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("plan =\n%+v\nwant\n%+v", plan, want)
	}
}

func TestParseScores_ReadsOnlyTheNamedSection(t *testing.T) {
	report := strings.Replace(fullScores(nil), "## "+ScoresSection, "## "+PlanSection, 1)
	if _, problems := ParseScores(report); len(problems) != 1 || !strings.Contains(problems[0], ScoresSection) {
		t.Errorf("a scores list under another heading: problems = %q, want one naming the missing ## %s", problems, ScoresSection)
	}
}
