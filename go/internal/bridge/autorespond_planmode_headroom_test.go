package bridge

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	headroomRuleName        = "plan_question"
	headroomNarrowPaneWidth = 100
	headroomWidePaneWidth   = 220
	headroomMinSlackLines   = 2
	headroomMaxWindowLines  = 30
	headroomOverLongLines   = 40
	headroomFooter          = "  tab to add notes | enter to submit answer | ←/→ to navigate questions | esc to interrupt"
	headroomLeadLine        = "• Need one detail before planning."
)

type headroomOption struct{ label, detail string }

type headroomForm struct {
	name  string
	lines []string
}

var headroomFourOptions = []headroomOption{
	{`Return "hello" (Recommended)`, "Implements `func greet() string` using the repository's only existing greeting text."},
	{`Print "hello"`, "Implements `func greet()` with stdout as its observable behavior."},
	{`Return "Hello, World!"`, "Implements the common kata-style string contract."},
	{"None of the above", "Optionally, add details in notes (tab)."},
}

var headroomFiveOptions = []headroomOption{
	headroomFourOptions[0],
	headroomFourOptions[1],
	headroomFourOptions[2],
	{`Return a greeting for a name`, "Accepts an optional name argument and falls back to hello when the name is empty."},
	headroomFourOptions[3],
}

const headroomShortQuestion = "What should the no-argument `greet()` function do?"

const headroomLongQuestion = "What should the no-argument `greet()` function do when the repository has no existing greeting convention, no tests, and no go.mod to anchor the package?"

func headroomWrap(lead, indent, text string, width int) []string {
	var rows []string
	row := lead
	started := false
	for _, word := range strings.Fields(text) {
		if started && utf8.RuneCountInString(row)+1+utf8.RuneCountInString(word) > width {
			rows = append(rows, row)
			row = indent + word
			continue
		}
		if started {
			row += " " + word
		} else {
			row += word
			started = true
		}
	}
	return append(rows, row)
}

func headroomPickerLines(question string, options []headroomOption, width int) []string {
	lines := []string{"  Question 1/2 (2 unanswered)"}
	lines = append(lines, headroomWrap("  ", "  ", question, width)...)
	for i, option := range options {
		lead := "    "
		if i == 0 {
			lead = "  › "
		}
		lead = fmt.Sprintf("%s%d. ", lead, i+1)
		lines = append(lines, headroomWrap(lead, "       ", option.label+"  "+option.detail, width)...)
	}
	return append(lines, headroomFooter)
}

func headroomPane(pickerLines []string) string {
	return strings.Join(append([]string{headroomLeadLine}, pickerLines...), "\n")
}

func headroomLiveForms() []headroomForm {
	return []headroomForm{
		{"four options, production pane width", headroomPickerLines(headroomShortQuestion, headroomFourOptions, headroomWidePaneWidth)},
		{"five options, production pane width", headroomPickerLines(headroomShortQuestion, headroomFiveOptions, headroomWidePaneWidth)},
		{"four options, wrapped at a narrow pane", headroomPickerLines(headroomShortQuestion, headroomFourOptions, headroomNarrowPaneWidth)},
		{"five options, wrapped at a narrow pane", headroomPickerLines(headroomShortQuestion, headroomFiveOptions, headroomNarrowPaneWidth)},
		{"five options, wrapped question and options", headroomPickerLines(headroomLongQuestion, headroomFiveOptions, headroomNarrowPaneWidth)},
	}
}

func headroomOverLongForm() headroomForm {
	lines := []string{"  Question 1/2 (2 unanswered)"}
	for i := 0; len(lines) < headroomOverLongLines-len(headroomFourOptions)-1; i++ {
		lines = append(lines, fmt.Sprintf("  context row %02d", i))
	}
	for i, option := range headroomFourOptions {
		lead := "    "
		if i == 0 {
			lead = "  › "
		}
		lines = append(lines, fmt.Sprintf("%s%d. %s", lead, i+1, option.label))
	}
	return headroomForm{"over-long question", append(lines, headroomFooter)}
}

func headroomManifestRules(t *testing.T) ([]ManifestPrompt, ManifestPrompt) {
	t.Helper()
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(codex-tmux): %v", err)
	}
	for _, p := range m.InteractivePrompts {
		if p.Name == headroomRuleName {
			return m.InteractivePrompts, p
		}
	}
	t.Fatalf("codex-tmux declares no %s rule", headroomRuleName)
	return nil, ManifestPrompt{}
}

func TestCodexPlanRuleTailHeadroom_MatchesFiveOptionAndWrappedForms(t *testing.T) {
	t.Parallel()
	prompts, _ := headroomManifestRules(t)
	for _, form := range headroomLiveForms() {
		t.Run(form.name, func(t *testing.T) {
			pane := headroomPane(form.lines)
			action, rc := decideAutoRespond(pane, prompts, map[string]int{}, true)
			if action != "send:Enter" || rc != 1 {
				t.Fatalf("live picker of %d rows went unanswered: got (%q, %d), want (\"send:Enter\", 1)\npane:\n%s",
					len(form.lines), action, rc, pane)
			}
		})
	}
}

func TestCodexPlanRuleTailHeadroom_WindowLeavesSlackOverTheLargestForm(t *testing.T) {
	t.Parallel()
	_, rule := headroomManifestRules(t)
	largest := 0
	for _, form := range headroomLiveForms() {
		if len(form.lines) > largest {
			largest = len(form.lines)
		}
	}
	if rule.TailLines-largest < headroomMinSlackLines {
		t.Fatalf("tail_lines %d leaves %d rows of slack over the largest %d-row picker, want at least %d",
			rule.TailLines, rule.TailLines-largest, largest, headroomMinSlackLines)
	}
}

func TestCodexPlanRuleTailHeadroom_WindowStaysBounded(t *testing.T) {
	t.Parallel()
	_, rule := headroomManifestRules(t)
	if rule.TailLines <= 0 || rule.TailLines > headroomMaxWindowLines {
		t.Fatalf("tail_lines %d must stay inside (0, %d] so the rule reads the live screen only",
			rule.TailLines, headroomMaxWindowLines)
	}
}

func TestCodexPlanRuleTailHeadroom_OverLongFormIsUnmatched(t *testing.T) {
	t.Parallel()
	prompts, _ := headroomManifestRules(t)
	form := headroomOverLongForm()
	if len(form.lines) < headroomOverLongLines {
		t.Fatalf("fixture has %d rows, want at least %d", len(form.lines), headroomOverLongLines)
	}
	pane := headroomPane(form.lines)
	action, rc := decideAutoRespond(pane, prompts, map[string]int{}, true)
	if action != "noop" || rc != 0 {
		t.Fatalf("a %d-row picker outside any sane window fired the rule: got (%q, %d), want (\"noop\", 0)",
			len(form.lines), action, rc)
	}
}

func TestCodexPlanRuleTailHeadroom_AnsweredFormsStayUnmatched(t *testing.T) {
	t.Parallel()
	prompts, _ := headroomManifestRules(t)
	below := [][]string{
		{"› "},
		{"• Ran go build ./...", "  └ ok"},
	}
	for _, form := range headroomLiveForms() {
		for _, extra := range below {
			t.Run(fmt.Sprintf("%s, %d rows below", form.name, len(extra)), func(t *testing.T) {
				pane := headroomPane(form.lines) + "\n" + strings.Join(extra, "\n")
				action, rc := decideAutoRespond(pane, prompts, map[string]int{}, true)
				if action != "noop" || rc != 0 {
					t.Fatalf("answered picker re-fired: got (%q, %d), want (\"noop\", 0)\npane:\n%s", action, rc, pane)
				}
			})
		}
	}
}

func TestCodexPlanRuleTailHeadroom_FixturesFitTheSpanBoundWithoutAWindow(t *testing.T) {
	t.Parallel()
	prompts, rule := headroomManifestRules(t)
	unwindowed := make([]ManifestPrompt, 0, len(prompts))
	for _, p := range prompts {
		if p.Name == rule.Name {
			p.TailLines = 0
		}
		unwindowed = append(unwindowed, p)
	}
	forms := append(headroomLiveForms(), headroomOverLongForm())
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			pane := headroomPane(form.lines)
			action, rc := decideAutoRespond(pane, unwindowed, map[string]int{}, true)
			if action != "send:Enter" || rc != 1 {
				t.Fatalf("fixture is not a valid picker for the rule's regex even with no window: got (%q, %d)\npane:\n%s",
					action, rc, pane)
			}
		})
	}
}
