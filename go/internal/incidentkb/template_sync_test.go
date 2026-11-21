package incidentkb_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

const (
	templatesRel  = "skills/docs-write/references/templates.md"
	issueSkillRel = "skills/issue-record/SKILL.md"
	templateFence = "````"
	templateID    = "2026-11-21-template-sync"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err == nil {
			return string(raw)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above the test directory", rel)
		}
		dir = parent
	}
}

func incidentTemplate(t *testing.T) string {
	t.Helper()
	doc := repoFile(t, templatesRel)
	_, afterTitle, ok := strings.Cut(doc, "\n## Incident\n")
	if !ok {
		t.Fatalf("%s has no \"## Incident\" section", templatesRel)
	}
	_, afterOpen, ok := strings.Cut(afterTitle, templateFence+"markdown\n")
	if !ok {
		t.Fatalf("the Incident section has no %smarkdown fence", templateFence)
	}
	block, _, ok := strings.Cut(afterOpen, "\n"+templateFence+"\n")
	if !ok {
		t.Fatalf("the Incident template fence is not closed by %s", templateFence)
	}
	return block + "\n"
}

func filledTemplate(t *testing.T) string {
	t.Helper()
	return strings.NewReplacer(
		"<yyyy-mm-dd-slug>", templateID,
		"<what broke, as a sentence>", "a sentence about what broke",
		"<cycle:N:S>", "cycle:1:1",
	).Replace(incidentTemplate(t))
}

func headingLines(doc string) []string {
	var out []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "## ") {
			out = append(out, line)
		}
	}
	return out
}

func TestIncidentTemplate_HeadingsEqualRequiredSections(t *testing.T) {
	t.Parallel()
	want := make([]string, 0, len(incidentkb.RequiredSections))
	for _, s := range incidentkb.RequiredSections {
		want = append(want, "## "+s)
	}
	if got := headingLines(incidentTemplate(t)); !slices.Equal(got, want) {
		t.Errorf("template headings = %q, want exactly %q", got, want)
	}
}

func TestIncidentTemplate_AFilledTemplateRendersToARecordTheCheckerAccepts(t *testing.T) {
	t.Parallel()
	rendered, err := incidentkb.Render([]byte(filledTemplate(t)))
	if err != nil {
		t.Fatalf("Render(filled template): %v", err)
	}
	if problems := incidentkb.CheckBytes(templateID, rendered); len(problems) != 0 {
		t.Fatalf("control: CheckBytes(rendered template) = %q, want none", problems)
	}
	broken := strings.Replace(string(rendered), "## Approaches taken\n", "", 1)
	problems := incidentkb.CheckBytes(templateID, []byte(broken))
	if len(problems) != 1 || !strings.Contains(problems[0], `missing section heading "## Approaches taken"`) {
		t.Errorf("without section 7, CheckBytes = %q, want exactly the missing Approaches taken heading", problems)
	}
}

func TestIncidentTemplate_AnUnrenderedTemplateFailsOnlyOnItsGeneratedParts(t *testing.T) {
	t.Parallel()
	problems := incidentkb.CheckBytes(templateID, []byte(filledTemplate(t)))
	if len(problems) != 3 {
		t.Fatalf("CheckBytes(unrendered template) = %q, want 3 rendered-part problems", problems)
	}
	for _, p := range problems {
		if !strings.Contains(p, "does not match frontmatter") {
			t.Errorf("problem %q is not a rendered-part mismatch", p)
		}
	}
}

var skillSectionCountRE = regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten|\d+) sections\b`)

func procedureActions(t *testing.T, skill string) map[string]string {
	t.Helper()
	rowRE := regexp.MustCompile(`^\| \d+\. ([^|]+) \| ([^|]+) \|`)
	actions := map[string]string{}
	for _, line := range strings.Split(skill, "\n") {
		if m := rowRE.FindStringSubmatch(line); m != nil {
			actions[strings.TrimSpace(m[1])] = m[2]
		}
	}
	return actions
}

func TestIssueRecordSkill_ProcedureRunsTheRenderAndCheckVerbs(t *testing.T) {
	t.Parallel()
	skill := repoFile(t, issueSkillRel)
	actions := procedureActions(t, skill)
	if got := actions["Write"]; !strings.Contains(got, "`evolve issue render <id>`") {
		t.Errorf("the Write step = %q, want it to run `evolve issue render <id>`", got)
	}
	if got := actions["Check"]; !strings.Contains(got, "`evolve issue check <id>`") {
		t.Errorf("the Check step = %q, want it to run `evolve issue check <id>`", got)
	}
	if m := skillSectionCountRE.FindString(skill); m != "" {
		t.Errorf("the skill states a section count %q; RequiredSections is the one home of it", m)
	}
	if strings.Contains(skill, "evolve issue stub") {
		t.Errorf("the skill names `evolve issue stub`, which the plan replaced with `evolve issue open`")
	}
}
