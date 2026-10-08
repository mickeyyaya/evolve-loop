package stelint

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	listItemPattern  = regexp.MustCompile(`^\s*([-*+]|\d{1,9}[.)])\s+(.*)$`)
	checkboxPattern  = regexp.MustCompile(`^\[[ xX]\]\s+`)
	separatorPattern = regexp.MustCompile(`^:?-+:?$`)
	headingPattern   = regexp.MustCompile(`^#{1,6}(\s|$)`)
	fileMarkers      = []string{"GENERATED", "DO NOT EDIT", "Code generated"}
)

const regionMarker = "GENERATED:"

type srcLine struct {
	n    int
	text string
}

type paragraphKind int

const (
	proseParagraph paragraphKind = iota
	stepParagraph
	headingParagraph
)

type paragraph struct {
	lines []srcLine
	kind  paragraphKind
}

func (p paragraph) atoms() []atom {
	var runes []rune
	var lines []int
	for k, l := range p.lines {
		if k > 0 {
			runes = append(runes, '\n')
			lines = append(lines, l.n)
		}
		for _, r := range l.text {
			runes = append(runes, r)
			lines = append(lines, l.n)
		}
	}
	return scanText(runes, lines, markdownSyntax)
}

func Check(doc []byte, opt Options) []Finding {
	lines := strings.Split(string(doc), "\n")
	if n, ok := generatedHeader(lines); ok {
		return []Finding{generatedNotice(n, "this file")}
	}
	b := splitBlocks(lines, opt.IsStandard)
	m := newMatcher(opt.Words)
	var out []Finding
	if len(b.regions) > 0 {
		out = append(out, generatedNotice(b.regions[0], fmt.Sprintf("%d marked region(s) of this file", len(b.regions))))
	}
	for _, p := range b.paragraphs {
		out = append(out, m.checkParagraph(p)...)
	}
	sortFindings(out)
	return out
}

func (m matcher) checkParagraph(p paragraph) []Finding {
	sents := m.sentences(p.atoms())
	out := m.wordFindingsOf(sents)
	if p.kind == headingParagraph {
		return out
	}
	limit := maxSentenceWords
	if p.kind == stepParagraph {
		limit = maxStepWords
	}
	out = append(out, lengthFindings(sents, limit)...)
	if f, ok := paragraphFinding(sents); ok {
		out = append(out, f)
	}
	return out
}

func frontmatterEnd(lines []string) int {
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if l := strings.TrimRight(lines[i], "\r"); l == "---" || l == "..." {
			return i + 1
		}
	}
	return 0
}

func generatedHeader(lines []string) (int, bool) {
	for i := frontmatterEnd(lines); i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "<!--") {
			return 0, false
		}
		for ; i < len(lines); i++ {
			if isFileMarker(lines[i]) {
				return i + 1, true
			}
			if strings.Contains(lines[i], "-->") {
				break
			}
		}
	}
	return 0, false
}

func isFileMarker(line string) bool {
	if strings.Contains(line, regionMarker) {
		return false
	}
	for _, m := range fileMarkers {
		if strings.Contains(line, m) {
			return true
		}
	}
	return false
}

func isRegionBegin(trimmed string) bool {
	return strings.HasPrefix(trimmed, "<!--") && strings.Contains(trimmed, regionMarker) && strings.Contains(trimmed, " BEGIN")
}

func isRegionEnd(trimmed string) bool {
	return strings.HasPrefix(trimmed, "<!--") && strings.Contains(trimmed, regionMarker) && strings.Contains(trimmed, " END")
}

func isHeading(trimmed string) bool { return headingPattern.MatchString(trimmed) }

func headingText(trimmed string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimLeft(trimmed, "#"), "# "))
}

func fenceMarker(trimmed string) string {
	for _, c := range []rune{'`', '~'} {
		if n := runLength([]rune(trimmed), 0, c); n >= 3 {
			return strings.Repeat(string(c), n)
		}
	}
	return ""
}

func closesFence(trimmed, fence string) bool {
	return strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == ""
}

func isHTMLBlockStart(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "<") {
		return false
	}
	end := strings.IndexAny(trimmed, "> ")
	if end < 0 {
		end = len(trimmed)
	}
	return blockTags[tagName(trimmed[1:end])]
}

func isRuleLine(trimmed string, marks string) bool {
	compact := strings.ReplaceAll(trimmed, " ", "")
	if len(compact) < 3 {
		return false
	}
	for _, c := range marks {
		if strings.Trim(compact, string(c)) == "" {
			return true
		}
	}
	return false
}

func listItem(text string) (paragraph, bool) {
	m := listItemPattern.FindStringSubmatch(text)
	if m == nil {
		return paragraph{}, false
	}
	kind := proseParagraph
	if marker := m[1]; marker[0] >= '0' && marker[0] <= '9' {
		kind = stepParagraph
	}
	return paragraph{kind: kind, lines: []srcLine{{text: checkboxPattern.ReplaceAllString(m[2], "")}}}, true
}

func quoteContent(trimmed string) string {
	for strings.HasPrefix(trimmed, ">") {
		trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, ">"), " ")
	}
	return trimmed
}

func startsWithQuotation(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	_, ok := quotePairs[r]
	return ok
}

func splitCells(row string) []string {
	rs := []rune(row)
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(rs); i++ {
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			cur.WriteString(string(rs[i : i+2]))
			i++
		case rs[i] == '`':
			end, _ := backtickSpanEnd(rs, i)
			cur.WriteString(string(rs[i:end]))
			i = end - 1
		case rs[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(rs[i])
		}
	}
	cells = append(cells, cur.String())
	if strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if !separatorPattern.MatchString(strings.TrimSpace(c)) {
			return false
		}
	}
	return len(cells) > 0
}
