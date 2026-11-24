package incidentkb_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/incidentkb"
)

const (
	templatePath = "skills/docs-write/references/templates.md"
	skillPath    = "skills/issue-record/SKILL.md"
	fixtureID    = "2026-11-25-template-fixture"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func incidentTemplate(t *testing.T) string {
	t.Helper()
	text := repoFile(t, templatePath)
	_, after, ok := strings.Cut(text, "\n## Incident\n")
	if !ok {
		t.Fatalf("%s has no Incident section", templatePath)
	}
	_, block, ok := strings.Cut(after, "```markdown\n")
	if !ok {
		t.Fatal("the Incident section has no markdown block")
	}
	template, _, ok := strings.Cut(block, "\n```")
	if !ok {
		t.Fatal("the Incident block is not closed")
	}
	return template + "\n"
}

func filledTemplate(t *testing.T) string {
	t.Helper()
	return strings.NewReplacer("<id>", fixtureID, "<title>", "template fixture", "<level>", "record", "<status>", "open").Replace(incidentTemplate(t))
}

func TestTemplate_TheIncidentBlockHasTheTenSectionsInOrder(t *testing.T) {
	t.Parallel()
	var headings []string
	for _, line := range strings.Split(incidentTemplate(t), "\n") {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, strings.TrimPrefix(line, "## "))
		}
	}
	if !slices.Equal(headings, incidentkb.RequiredSections) {
		t.Fatalf("template headings %v, want %v", headings, incidentkb.RequiredSections)
	}
}

func TestTemplate_FilledAndRenderedItPassesTheCheck(t *testing.T) {
	t.Parallel()
	rendered, err := incidentkb.RenderFor(fixtureID, []byte(filledTemplate(t)))
	if err != nil {
		t.Fatalf("the template does not render: %v", err)
	}
	if problems := incidentkb.CheckBytes(fixtureID, rendered); len(problems) != 0 {
		t.Fatalf("the rendered template fails the check: %v", problems)
	}
}

func TestTemplate_ARecordThatLacksSectionSevenFails(t *testing.T) {
	t.Parallel()
	broken := strings.Replace(filledTemplate(t), "## Approaches taken\n", "", 1)
	rendered, err := incidentkb.RenderFor(fixtureID, []byte(broken))
	if err != nil {
		t.Fatal(err)
	}
	problems := incidentkb.CheckBytes(fixtureID, rendered)
	if len(problems) != 1 || !strings.Contains(problems[0], `missing section heading "## Approaches taken"`) {
		t.Fatalf("problems = %v", problems)
	}
}

func TestSkill_TheProcedureNamesTheRenderAndCheckVerbs(t *testing.T) {
	t.Parallel()
	text := repoFile(t, skillPath)
	for _, want := range []string{"evolve issue render", "evolve issue check", "## Key logs", "evolve issue open"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not mention %q", skillPath, want)
		}
	}
	for _, stale := range []string{"The five sections exist", "evolve issue stub"} {
		if strings.Contains(text, stale) {
			t.Errorf("%s still says %q", skillPath, stale)
		}
	}
}

func TestRealTree_EveryRecordPassesTheCheck(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	report, err := incidentkb.CheckAll(root)
	if err != nil {
		t.Fatal(err)
	}
	for id, problems := range report.Problems {
		if len(problems) != 0 {
			t.Errorf("%s: %v", id, problems)
		}
	}
}
