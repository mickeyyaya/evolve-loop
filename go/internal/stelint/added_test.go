package stelint

import (
	"reflect"
	"strings"
	"testing"
)

func TestKeepAdded_KeepsAFindingWhoseBlockHoldsAnAddedLine(t *testing.T) {
	t.Parallel()
	raw := []byte("# Title\n\nold text\nnew text\n\n- item one\n- item two\n\n| a | b |\n")
	findings := []Finding{{Line: 3, Rule: "A"}, {Line: 6, Rule: "B"}, {Line: 7, Rule: "C"}, {Line: 9, Rule: "D"}, {Line: 0, Rule: "SKIP"}}
	got := KeepAdded(raw, false, map[int]bool{4: true, 7: true}, findings)
	want := []Finding{{Line: 3, Rule: "A"}, {Line: 7, Rule: "C"}, {Line: 0, Rule: "SKIP"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kept = %+v, want %+v: a paragraph holding an added line keeps its finding; a list item and a table row stand alone", got, want)
	}
	if got := KeepAdded(raw, true, map[int]bool{4: true}, findings); !reflect.DeepEqual(got, []Finding{{Line: 0, Rule: "SKIP"}}) {
		t.Errorf("exact lines kept = %+v, want only the skip notice: line 3 is not added", got)
	}
}

func TestBlockSpan(t *testing.T) {
	t.Parallel()
	lines := strings.Split("# Head\n\nfirst\nsecond\n- item one\n  more\n- item two\n\n| a | b |\nlast\n", "\n")
	for name, tc := range map[string]struct {
		line, from, to int
	}{
		"a heading is alone":                     {1, 1, 1},
		"a blank line is alone":                  {2, 2, 2},
		"a paragraph runs to the next list item": {3, 3, 4},
		"a list item runs to its continuation":   {5, 5, 6},
		"a continuation joins its item":          {6, 5, 6},
		"the next item stands alone":             {7, 7, 7},
		"a table row is alone":                   {9, 9, 9},
		"a line after a table row is its own":    {10, 10, 10},
		"a line before the file is itself":       {0, 0, 0},
		"a line past the file is itself":         {99, 99, 99},
	} {
		t.Run(name, func(t *testing.T) {
			from, to := blockSpan(lines, tc.line)
			if got, want := []int{from, to}, []int{tc.from, tc.to}; !reflect.DeepEqual(got, want) {
				t.Fatalf("blockSpan(%d) = %v, want %v", tc.line, got, want)
			}
		})
	}
}

func TestBlockSpan_AQuotedListItemStandsAlone(t *testing.T) {
	t.Parallel()
	lines := strings.Split("> **Head:**\n>\n> - item one\n>   more\n> - item two\n> - item three\n>\n> quoted text\n", "\n")
	for name, tc := range map[string]struct {
		line, from, to int
	}{
		"a quoted item runs to its continuation": {3, 3, 4},
		"a quoted item stands alone":             {5, 5, 5},
		"the last quoted item stops at a bare >": {6, 6, 6},
		"a bare > is alone":                      {7, 7, 7},
		"quoted text after a bare > is its own":  {8, 8, 8},
	} {
		t.Run(name, func(t *testing.T) {
			from, to := blockSpan(lines, tc.line)
			if got, want := []int{from, to}, []int{tc.from, tc.to}; !reflect.DeepEqual(got, want) {
				t.Fatalf("blockSpan(%d) = %v, want %v", tc.line, got, want)
			}
		})
	}
}

func FuzzBlockSpan(f *testing.F) {
	f.Add("a\n\nb\n- c\n| d |\n# e\n", 3)
	f.Add("", 1)
	f.Add("x", -4)
	f.Fuzz(func(t *testing.T, text string, line int) {
		lines := strings.Split(text, "\n")
		from, to := blockSpan(lines, line)
		if from > to || from > line || to < line {
			t.Fatalf("blockSpan(%q, %d) = %d, %d; want a span that holds the line", text, line, from, to)
		}
		if line >= 1 && line <= len(lines) && (from < 1 || to > len(lines)) {
			t.Fatalf("blockSpan(%q, %d) = %d, %d; want a span inside the file", text, line, from, to)
		}
	})
}
