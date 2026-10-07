package panestream

import (
	"strings"
	"testing"
)

const testModelLabelRegex = `^(?:esc to cancel|\? for shortcuts)?\s{2,}(?P<model>\S[^·]*·\s*\S+)\s*$`

func TestModelLabel_ReadsTheFooterLabelOfTheLastLine(t *testing.T) {
	cases := map[string]string{
		"idle.txt":         "Gemini 3.8 Flash · low",
		"editing-a.txt":    "Gemini 3.8 Flash · low",
		"answer.txt":       "Gemini 3.8 Flash · high",
		"parked-paste.txt": "Gemini 3.8 Flash · low",
		"generating-a.txt": "",
	}
	for name, want := range cases {
		if got := ModelLabel(testdataFrame(t, "agy-1.2.17/"+name), testModelLabelRegex); got != want {
			t.Errorf("%s: model label %q, want %q", name, got, want)
		}
	}
	if ModelLabel("? for shortcuts      Gemini · low", "") != "" {
		t.Error("an empty pattern reads no label")
	}
	if ModelLabel("x", `(?P<other>x)`) != "" {
		t.Error("a pattern without a model group reads no label")
	}
	if ModelLabel("\n\n", testModelLabelRegex) != "" {
		t.Error("a blank pane has no label")
	}
}

const testFooterLabelRegex = `^(?P<footer>esc to cancel|\? for shortcuts)?\s{2,}(?P<model>[[:alnum:]].*)$`

func TestFooterModelLabel_ReadsOnlyTheBottomMostFooterPrefixLine(t *testing.T) {
	rule := strings.Repeat("─", 30)
	stale := "? for shortcuts      Claude Opus 5.5 · high\n" + rule + "\n>\n" + rule + "\n"
	for name, tc := range map[string]struct {
		pane string
		tail int
		want string
	}{
		"a label-shaped toast under the footer":    {"  Claude Opus 5.5 · high\n" + rule + "\n>\n" + rule + "\n? for shortcuts      Gemini 3.8 Flash · low\n\n  Claude Opus 5.5 ready\n", 6, "Gemini 3.8 Flash · low"},
		"the busy footer":                          {"  output\nesc to cancel      Gemini 3.1 Pro · high", 6, "Gemini 3.1 Pro · high"},
		"no footer prefix in the tail":             {"  Claude Opus 5.5 · high\n" + rule + "\n>\n" + rule + "\n  Gemini 3.8 Flash · low", 6, ""},
		"a stale footer over a labelled footer":    {stale + "? for shortcuts      Gemini 3.8 Flash · low", 6, "Gemini 3.8 Flash · low"},
		"a stale footer over an unlabelled footer": {stale + "? for shortcuts", 6, ""},
		"a footer above the tail":                  {"? for shortcuts      Claude Opus 5.5 · high\n  a\n  b", 2, ""},
		"a footer at the top of the tail":          {"? for shortcuts      Claude Opus 5.5 · high\n  a\n  b", 3, "Claude Opus 5.5 · high"},
		"an ANSI-coloured footer":                  {"\x1b[2m? for shortcuts\x1b[0m      \x1b[36mClaude Opus 5.5 · high\x1b[0m", 6, "Claude Opus 5.5 · high"},
		"the prefix text in the middle of a line":  {"  press ? for shortcuts      Claude Opus 5.5 · high", 6, ""},
	} {
		if got := FooterModelLabel(tc.pane, testFooterLabelRegex, tc.tail); got != tc.want {
			t.Errorf("%s: label %q, want %q", name, got, tc.want)
		}
	}
}

func TestFooterModelLabel_APatternWithoutBothNamedGroupsReadsNoLabel(t *testing.T) {
	for name, pattern := range map[string]string{
		"no footer group":  `^(?:\? for shortcuts)?\s{2,}(?P<model>[[:alnum:]].*)$`,
		"no model group":   `^(?P<footer>\? for shortcuts)?\s{2,}([[:alnum:]].*)$`,
		"an unparseable":   `^(?P<footer>\? for shortcuts\s{2,}(?P<model>.*)$`,
		"an empty pattern": "",
	} {
		if got := FooterModelLabel("? for shortcuts      Claude Opus 5.5 · high", pattern, 6); got != "" {
			t.Errorf("%s pattern %q read the label %q", name, pattern, got)
		}
	}
}
