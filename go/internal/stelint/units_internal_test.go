package stelint

import (
	"go/ast"
	"go/token"
	"reflect"
	"testing"
)

func importSpec(name, path string) *ast.ImportSpec {
	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: path}}
	if name != "" {
		spec.Name = ast.NewIdent(name)
	}
	return spec
}

func TestImportNames_MapsEachLocalNameAndSkipsABrokenPath(t *testing.T) {
	file := &ast.File{Imports: []*ast.ImportSpec{
		importSpec("", `"unterminated`),
		importSpec("", `"fmt"`),
		importSpec("", "`net/http`"),
		importSpec("stdlog", `"log"`),
		importSpec("", `"example.com/a/b/v2"`),
	}}

	got := importNames(file)

	want := map[string]string{"fmt": "fmt", "http": "net/http", "stdlog": "log", "v2": "example.com/a/b/v2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("importNames = %v, want %v", got, want)
	}
}

func TestImportNames_AFileWithoutImportsIsEmpty(t *testing.T) {
	if got := importNames(&ast.File{}); len(got) != 0 {
		t.Fatalf("importNames = %v, want an empty map", got)
	}
}

func TestSplitCells(t *testing.T) {
	for name, tc := range map[string]struct {
		row  string
		want []string
	}{
		"outer pipes are dropped":           {"| a | b |", []string{" a ", " b "}},
		"no outer pipes":                    {"a | b", []string{"a ", " b"}},
		"an escaped pipe stays":             {`| a \| b | c |`, []string{` a \| b `, " c "}},
		"a pipe in a code span stays":       {"| `a|b` | c |", []string{" `a|b` ", " c "}},
		"an unclosed code span keeps going": {"| `a | b |", []string{" `a ", " b "}},
		"a trailing backslash is kept":      {`| a \`, []string{` a \`}},
		"an empty cell in the middle stays": {"| a || c |", []string{" a ", "", " c "}},
		"a lone pipe is no cell":            {"|", []string{}},
		"an empty row is no cell":           {"", []string{}},
		"unicode text is kept whole":        {"| café | naïve |", []string{" café ", " naïve "}},
	} {
		t.Run(name, func(t *testing.T) {
			got := splitCells(tc.row)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitCells(%q) = %q, want %q", tc.row, got, tc.want)
			}
		})
	}
}

func TestBacktickSpanEnd(t *testing.T) {
	for name, tc := range map[string]struct {
		text   string
		at     int
		end    int
		closed bool
	}{
		"a single span":                     {"`a` b", 0, 3, true},
		"a double span skips a single":      {"``a`b`` c", 0, 7, true},
		"a single span skips a double":      {"`a``b` c", 0, 6, true},
		"an unclosed single":                {"`abc", 0, 1, false},
		"an unclosed double":                {"``a`b", 0, 2, false},
		"a span that starts later":          {"x `y` z", 2, 5, true},
		"a backtick at the end is unclosed": {"x `", 2, 3, false},
	} {
		t.Run(name, func(t *testing.T) {
			end, closed := backtickSpanEnd([]rune(tc.text), tc.at)
			if end != tc.end || closed != tc.closed {
				t.Fatalf("backtickSpanEnd(%q, %d) = %d, %v; want %d, %v", tc.text, tc.at, end, closed, tc.end, tc.closed)
			}
		})
	}
}

func TestFrontmatterEnd(t *testing.T) {
	for name, tc := range map[string]struct {
		lines []string
		want  int
	}{
		"no lines":                {nil, 0},
		"no frontmatter":          {[]string{"# Title", "---"}, 0},
		"a closed frontmatter":    {[]string{"---", "a: b", "---", "text"}, 3},
		"a dots close":            {[]string{"---", "a: b", "...", "text"}, 3},
		"an empty frontmatter":    {[]string{"---", "---"}, 2},
		"CRLF fences":             {[]string{"---\r", "a: b\r", "---\r"}, 3},
		"an unclosed frontmatter": {[]string{"---", "a: b", "text"}, 0},
		"an indented fence":       {[]string{" ---", "a: b", "---"}, 0},
	} {
		t.Run(name, func(t *testing.T) {
			if got := frontmatterEnd(tc.lines); got != tc.want {
				t.Fatalf("frontmatterEnd(%q) = %d, want %d", tc.lines, got, tc.want)
			}
		})
	}
}

func TestIsRuleLine(t *testing.T) {
	for name, tc := range map[string]struct {
		line, marks string
		want        bool
	}{
		"three dashes":           {"---", "-*_", true},
		"two dashes":             {"--", "-*_", false},
		"spaced stars":           {"* * *", "-*_", true},
		"two spaced stars":       {"* *", "-*_", false},
		"mixed marks":            {"-*-", "-*_", false},
		"equals for a heading":   {"===", "=-", true},
		"stars for a heading":    {"***", "=-", false},
		"text":                   {"abc", "-*_", false},
		"an empty line":          {"", "-*_", false},
		"a long underscore rule": {"__________", "-*_", true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := isRuleLine(tc.line, tc.marks); got != tc.want {
				t.Fatalf("isRuleLine(%q, %q) = %v, want %v", tc.line, tc.marks, got, tc.want)
			}
		})
	}
}

func TestIsHTMLBlockStart(t *testing.T) {
	for name, tc := range map[string]struct {
		line string
		want bool
	}{
		"an open div":           {"<div", true},
		"a div with attributes": {"<div class=x>", true},
		"a closing div":         {"</div>", true},
		"an uppercase table":    {"<TABLE>", true},
		"a self-closing hr":     {"<hr/>", true},
		"an inline span":        {"<span>", false},
		"an unknown tag":        {"<foo>", false},
		"a lone bracket":        {"<", false},
		"text":                  {"div", false},
		"an empty line":         {"", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := isHTMLBlockStart(tc.line); got != tc.want {
				t.Fatalf("isHTMLBlockStart(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestPieceEndsSentence(t *testing.T) {
	runes := func(s string) piece {
		p := make(piece, 0, len(s))
		for _, r := range s {
			p = append(p, atom{r: r})
		}
		return p
	}
	abbrevs := map[string]bool{"e.g.": true}
	for name, tc := range map[string]struct {
		p    piece
		want bool
	}{
		"a full stop":                        {runes("end."), true},
		"a question mark":                    {runes("end?"), true},
		"an exclamation mark in a bracket":   {runes("(end!)"), true},
		"a full stop before a closing quote": {runes("end.”"), true},
		"a listed abbreviation":              {runes("e.g."), false},
		"a listed abbreviation in a bracket": {runes("(e.g.)"), false},
		"only closing marks":                 {runes(")*_"), false},
		"an empty piece":                     {piece{}, false},
		"a comma":                            {runes("end,"), false},
		"a quotation with a stop":            {piece{{code: `"Stop."`, quotation: true}}, true},
		"a quotation without a stop":         {piece{{code: `"Stop"`, quotation: true}}, false},
		"a code span with a stop":            {piece{{code: "`a.`"}}, false},
		"a code span then a stop":            {append(piece{{code: "`a`"}}, atom{r: '.'}), true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.p.endsSentence(abbrevs); got != tc.want {
				t.Fatalf("endsSentence(%q) = %v, want %v", tc.p.text(), got, tc.want)
			}
		})
	}
}

func TestNewMatcher_SkipsEmptyPhrasesAndListsAbbreviations(t *testing.T) {
	m := newMatcher([]Substitution{{Phrase: ""}, {Phrase: " \t "}, {Phrase: "Et Al."}, {Phrase: "in order to"}})

	if len(m.phrases) != 2 || !reflect.DeepEqual(m.phrases[0].words, []string{"et", "al."}) || !reflect.DeepEqual(m.phrases[1].words, []string{"in", "order", "to"}) {
		t.Errorf("phrases = %+v, want et al. and in order to", m.phrases)
	}
	if !reflect.DeepEqual(m.abbrevs, map[string]bool{"al.": true}) {
		t.Errorf("abbrevs = %v, want only al.", m.abbrevs)
	}
}
