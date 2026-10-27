//go:build acs

package cycle2038

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stelint"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	operatingPolicyRel  = "docs/operations/operating-policy.md"
	runtimeReferenceRel = "docs/operations/runtime-reference.md"
	decideCommand       = "evolve convergence decide --input <rounds.json>"
	baseRef             = "refs/remotes/origin/main"
)

var (
	findingFields     = []string{"kind", "blocking", "component", "late"}
	headingPattern    = regexp.MustCompile(`^#{1,6}\s`)
	templateHeading   = regexp.MustCompile(`(?i)review[- ]brief`)
	afterEveryCheck   = regexp.MustCompile(`(?i)\bafter (every|each) review check\b`)
	followsDecision   = regexp.MustCompile(`(?i)\bfollow\w*\s+(the|its)\s+decision\b`)
	fullFirstJudgment = regexp.MustCompile(`(?i)\bfull\b`)
	verifyOnly        = regexp.MustCompile(`(?i)verify-only`)
	listItemStart     = regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s`)
	hunkHeader        = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)
)

type template struct {
	heading string
	body    string
}

func fieldKey(name string) *regexp.Regexp {
	return regexp.MustCompile("(?m)(^|[\\s\"'`{,-])" + name + "[\"'`]?\\s*:")
}

func proseUnits(doc string) []string {
	var units []string
	var current []string
	inFence := false
	flush := func() {
		if len(current) > 0 {
			units = append(units, strings.Join(current, " "))
			current = nil
		}
	}
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			flush()
			inFence = !inFence
			continue
		}
		switch {
		case inFence:
		case trimmed == "" || headingPattern.MatchString(trimmed):
			flush()
		case listItemStart.MatchString(line):
			flush()
			current = append(current, trimmed)
		default:
			current = append(current, trimmed)
		}
	}
	flush()
	return units
}

func reviewBriefTemplates(doc string) []template {
	var found []template
	heading := ""
	var body []string
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```") && !inFence:
			inFence = true
			body = nil
		case strings.HasPrefix(trimmed, "```") && inFence:
			inFence = false
			if templateHeading.MatchString(heading) {
				found = append(found, template{heading: heading, body: strings.Join(body, "\n")})
			}
		case inFence:
			body = append(body, line)
		case headingPattern.MatchString(trimmed):
			heading = trimmed
		}
	}
	return found
}

func consoleRuleViolations(doc string) []string {
	for _, unit := range proseUnits(doc) {
		if !strings.Contains(unit, decideCommand) {
			continue
		}
		var missing []string
		if !strings.Contains(strings.ToLower(unit), "console") {
			missing = append(missing, "the console as the actor")
		}
		if !afterEveryCheck.MatchString(unit) {
			missing = append(missing, "\"after every review check\"")
		}
		if !followsDecision.MatchString(unit) {
			missing = append(missing, "\"follow the decision\"")
		}
		return missing
	}
	return []string{"a prose sentence that names `" + decideCommand + "`"}
}

func judgmentRuleViolations(tpl template) []string {
	var missing []string
	if !strings.Contains(tpl.body, "J_0") || !fullFirstJudgment.MatchString(tpl.body) {
		missing = append(missing, "the full first judgment J_0")
	}
	if !verifyOnly.MatchString(tpl.body) {
		missing = append(missing, "verify-only later judgments")
	}
	return missing
}

func fieldViolations(tpl template) []string {
	var missing []string
	for _, name := range findingFields {
		if !fieldKey(name).MatchString(tpl.body) {
			missing = append(missing, "the finding field "+name)
		}
	}
	for _, value := range []convergence.Kind{convergence.KindDefect, convergence.KindCapability} {
		if !strings.Contains(tpl.body, string(value)) {
			missing = append(missing, "the kind value "+string(value))
		}
	}
	return missing
}

func roundsJSONFromTemplate(tpl template) []byte {
	finding := map[string]any{"id": "f1", "severity": "LOW", "status": "OPEN"}
	sample := map[string]any{"kind": string(convergence.KindDefect), "component": "pkg:part", "late": false}
	for key, value := range sample {
		if fieldKey(key).MatchString(tpl.body) {
			finding[key] = value
		}
	}
	doc := map[string]any{
		"loop":   "console-lane",
		"round":  0,
		"rounds": []any{map[string]any{"index": 0, "findings": []any{finding}}},
	}
	raw, _ := json.Marshal(doc)
	return raw
}

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func requireTemplates(t *testing.T) []template {
	t.Helper()
	templates := reviewBriefTemplates(readDoc(t, operatingPolicyRel))
	if len(templates) == 0 {
		t.Fatalf("RED: %s holds no review-brief template: add a heading with the words \"review brief\" and put each template in a fenced block under it", operatingPolicyRel)
	}
	return templates
}

func TestC2038_001_OperatingPolicyTellsConsoleToRunDecideAfterEveryCheck(t *testing.T) {
	if missing := consoleRuleViolations(readDoc(t, operatingPolicyRel)); len(missing) > 0 {
		t.Errorf("RED: %s needs %s", operatingPolicyRel, strings.Join(missing, ", "))
	}
}

func TestC2038_002_EveryReviewBriefAsksFullFirstJudgmentAndVerifyOnlyLater(t *testing.T) {
	for _, tpl := range requireTemplates(t) {
		if missing := judgmentRuleViolations(tpl); len(missing) > 0 {
			t.Errorf("RED: template under %q lacks %s", tpl.heading, strings.Join(missing, ", "))
		}
	}
}

func TestC2038_003_EveryReviewBriefAsksKindBlockingComponentLateForEachFinding(t *testing.T) {
	for _, tpl := range requireTemplates(t) {
		if missing := fieldViolations(tpl); len(missing) > 0 {
			t.Errorf("RED: template under %q lacks %s", tpl.heading, strings.Join(missing, ", "))
		}
	}
}

func TestC2038_004_TemplateFieldNamesAreAcceptedByTheConvergenceDecoder(t *testing.T) {
	for _, tpl := range requireTemplates(t) {
		in, err := convergence.Parse(roundsJSONFromTemplate(tpl))
		if err != nil {
			t.Errorf("template under %q: a finding with its field names is refused by the strict rounds decoder: %v", tpl.heading, err)
			continue
		}
		in.Config = policy.Policy{}.ConvergenceConfig()
		if err := in.Validate(); err != nil {
			t.Errorf("template under %q: the decoded rounds do not validate: %v", tpl.heading, err)
		}
		if got := len(in.Rounds[0].Findings); got != 1 {
			t.Errorf("template under %q: decoded %d findings, want 1", tpl.heading, got)
		}
	}
}

func addedLinesAgainstMain(t *testing.T, root, rel string) map[int]bool {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-U0", baseRef, "--", rel)
	if err != nil || code != 0 {
		t.Fatalf("git diff %s against %s: exit %d: %v %s", rel, baseRef, code, err, stderr)
	}
	added := map[int]bool{}
	for _, line := range strings.Split(out, "\n") {
		match := hunkHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		start, _ := strconv.Atoi(match[1])
		count := 1
		if match[2] != "" {
			count, _ = strconv.Atoi(match[2])
		}
		for n := start; n < start+count; n++ {
			added[n] = true
		}
	}
	return added
}

func TestC2038_005_ChangedDocsPassSteLintStrict(t *testing.T) {
	root := acsassert.RepoRoot(t)
	words, err := stelint.LoadStandard(root)
	if err != nil {
		t.Fatalf("load the STE standard: %v", err)
	}
	if len(addedLinesAgainstMain(t, root, operatingPolicyRel)) == 0 {
		t.Errorf("RED: %s is unchanged against %s: the change set has no doc to lint", operatingPolicyRel, baseRef)
	}
	for _, rel := range []string{operatingPolicyRel, runtimeReferenceRel} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		findings, err := stelint.LintFile(path, stelint.Options{Words: words})
		if err != nil {
			t.Fatalf("lint %s: %v", rel, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, f := range stelint.KeepAdded(raw, false, addedLinesAgainstMain(t, root, rel), findings) {
			if !f.IsSkipNotice() {
				t.Errorf("%s:%d %s: %s", rel, f.Line, f.Rule, f.Excerpt)
			}
		}
	}
}

func TestC2038_006_PredicateParsersRejectBriefsThatMissTheRule(t *testing.T) {
	complete := template{heading: "### Review brief", body: strings.Join([]string{
		"Give a full first judgment (J_0). Later judgments are verify-only.",
		"- kind: defect|capability",
		"- blocking: true|false",
		"- component: name",
		"- late: true|false",
	}, "\n")}
	if got := append(judgmentRuleViolations(complete), fieldViolations(complete)...); len(got) != 0 {
		t.Fatalf("a complete brief is flagged: %v", got)
	}
	withoutLate := template{body: strings.Replace(complete.body, "- late: true|false", "", 1)}
	if got := fieldViolations(withoutLate); len(got) != 1 || !strings.Contains(got[0], "late") {
		t.Errorf("a brief without late: violations = %v", got)
	}
	withoutVerifyOnly := template{body: strings.Replace(complete.body, "Later judgments are verify-only.", "", 1)}
	if got := judgmentRuleViolations(withoutVerifyOnly); len(got) != 1 {
		t.Errorf("a brief without verify-only: violations = %v", got)
	}
	withoutJ0 := template{body: strings.Replace(complete.body, "J_0", "round zero", 1)}
	if got := judgmentRuleViolations(withoutJ0); len(got) != 1 {
		t.Errorf("a brief without J_0: violations = %v", got)
	}
	fixture := "## Console review chain\n\n### Review brief\n\n```\n" + complete.body + "\n```\n"
	found := reviewBriefTemplates(fixture)
	if len(found) != 1 {
		t.Fatalf("a fenced template under a review-brief heading: found %d, want 1", len(found))
	}
	if _, err := convergence.Parse(roundsJSONFromTemplate(found[0])); err != nil {
		t.Errorf("the rounds JSON built from a complete template is refused: %v", err)
	}
	unrelated := "### Other heading\n\n```\n" + complete.body + "\n```\n"
	if got := reviewBriefTemplates(unrelated); len(got) != 0 {
		t.Errorf("a fenced block under an unrelated heading counts as a template")
	}
	insideFence := "```\nAfter every review check the console runs `" + decideCommand + "` and follows the decision.\n```\n"
	if got := consoleRuleViolations(insideFence); len(got) == 0 {
		t.Errorf("a rule that sits only inside a fenced block passes")
	}
	wrongTiming := "The console runs `" + decideCommand + "` once and follows the decision.\n"
	if got := consoleRuleViolations(wrongTiming); len(got) == 0 {
		t.Errorf("a rule without \"after every review check\" passes")
	}
	proper := "- After every review check, the console runs `" + decideCommand + "` and follows the decision.\n"
	if got := consoleRuleViolations(proper); len(got) != 0 {
		t.Errorf("a complete rule is flagged: %v", got)
	}
}
