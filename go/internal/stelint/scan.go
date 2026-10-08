package stelint

import (
	"strings"
	"unicode"
)

type syntaxHandler func(s *scanner, i, to int) (int, bool)

var markdownSyntax = map[rune]syntaxHandler{
	'<':  (*scanner).angle,
	'[':  (*scanner).link,
	'\\': (*scanner).escape,
}

var goTextSyntax = map[rune]syntaxHandler{
	'%': (*scanner).formatVerb,
}

var (
	inlineTags = setOf(strings.Fields("a abbr b br code del em font i img input ins kbd mark q s samp small span strong sub sup u var"))
	blockTags  = setOf(strings.Fields("blockquote center dd details div dl dt figcaption figure h1 h2 h3 h4 h5 h6 hr li ol p picture pre section source summary table tbody td tfoot th thead tr ul video"))
	quotePairs = map[rune]rune{'"': '"', '“': '”'}
)

func setOf(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

var urlSchemes = []string{"http://", "https://", "mailto:"}

type scanner struct {
	runes  []rune
	lines  []int
	syntax map[rune]syntaxHandler
	out    []atom
	exempt bool
}

func scanText(runes []rune, lines []int, syntax map[rune]syntaxHandler) []atom {
	s := &scanner{runes: runes, lines: lines, syntax: syntax}
	s.run(0, len(runes))
	return s.out
}

func (s *scanner) run(from, to int) {
	for i := from; i < to; {
		i = s.step(i, to)
	}
}

func (s *scanner) step(i, to int) int {
	r := s.runes[i]
	if r == '`' {
		return s.codeSpan(i, to)
	}
	if next, ok := s.quotation(i, to); ok {
		return next
	}
	if h, ok := s.syntax[r]; ok {
		if next, handled := h(s, i, to); handled {
			return next
		}
	}
	s.emit(i)
	return i + 1
}

func (s *scanner) quotation(i, to int) (int, bool) {
	closing, ok := quotePairs[s.runes[i]]
	if !ok {
		return 0, false
	}
	for j := i + 1; j < to; j++ {
		if s.runes[j] == closing {
			s.out = append(s.out, atom{code: string(s.runes[i : j+1]), quotation: true, line: s.lines[i]})
			return j + 1, true
		}
	}
	return 0, false
}

func (s *scanner) emit(i int) {
	s.out = append(s.out, atom{r: s.runes[i], line: s.lines[i], exempt: s.exempt})
}

func (s *scanner) emitCode(from, to int) {
	s.out = append(s.out, atom{code: string(s.runes[from:to]), line: s.lines[from]})
}

func (s *scanner) codeSpan(i, to int) int {
	end, closed := backtickSpanEnd(s.runes[:to], i)
	if closed {
		s.emitCode(i, end)
		return end
	}
	for k := i; k < end; k++ {
		s.emit(k)
	}
	return end
}

func backtickSpanEnd(rs []rune, i int) (int, bool) {
	n := runLength(rs, i, '`')
	for j := i + n; j < len(rs); {
		if rs[j] != '`' {
			j++
			continue
		}
		m := runLength(rs, j, '`')
		if m == n {
			return j + m, true
		}
		j += m
	}
	return i + n, false
}

func runLength(rs []rune, i int, r rune) int {
	n := 0
	for i+n < len(rs) && rs[i+n] == r {
		n++
	}
	return n
}

func (s *scanner) hasPrefixAt(i, to int, prefix []rune) bool {
	if i+len(prefix) > to {
		return false
	}
	for k, r := range prefix {
		if s.runes[i+k] != r {
			return false
		}
	}
	return true
}

func (s *scanner) indexFrom(i, to int, needle string) int {
	n := []rune(needle)
	for j := i; j < to; j++ {
		if s.hasPrefixAt(j, to, n) {
			return j
		}
	}
	return -1
}

func (s *scanner) angle(i, to int) (int, bool) {
	if s.hasPrefixAt(i, to, []rune("<!--")) {
		end := s.indexFrom(i+4, to, "-->")
		if end < 0 {
			return to, true
		}
		return end + 3, true
	}
	end := s.indexFrom(i+1, to, ">")
	if end < 0 {
		return 0, false
	}
	inner := string(s.runes[i+1 : end])
	for _, scheme := range urlSchemes {
		if strings.HasPrefix(inner, scheme) {
			s.emitCode(i, end+1)
			return end + 1, true
		}
	}
	if name := tagName(inner); inlineTags[name] || blockTags[name] {
		return end + 1, true
	}
	return 0, false
}

func tagName(inner string) string {
	name := strings.TrimPrefix(inner, "/")
	if k := strings.IndexFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '/' }); k >= 0 {
		name = name[:k]
	}
	return strings.ToLower(name)
}

func (s *scanner) link(i, to int) (int, bool) {
	closeText := s.matching(i, to, '[', ']')
	if closeText < 0 || closeText+1 >= to {
		return 0, false
	}
	var end int
	switch s.runes[closeText+1] {
	case '(':
		end = s.matching(closeText+1, to, '(', ')')
	case '[':
		end = s.matching(closeText+1, to, '[', ']')
	default:
		return 0, false
	}
	if end < 0 {
		return 0, false
	}
	outer := s.exempt
	s.exempt = true
	s.run(i+1, closeText)
	s.exempt = outer
	return end + 1, true
}

func (s *scanner) matching(open, to int, l, r rune) int {
	depth := 0
	for j := open; j < to; j++ {
		switch s.runes[j] {
		case l:
			depth++
		case r:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

func (s *scanner) escape(i, to int) (int, bool) {
	if i+1 >= to || (!unicode.IsPunct(s.runes[i+1]) && !unicode.IsSymbol(s.runes[i+1])) {
		return 0, false
	}
	s.emit(i + 1)
	return i + 2, true
}

func (s *scanner) formatVerb(i, to int) (int, bool) {
	j := i + 1
	if j < to && s.runes[j] == '%' {
		s.emit(i)
		return j + 1, true
	}
	j = s.skipWhile(j, to, func(r rune) bool { return strings.ContainsRune("+-#0", r) })
	j = s.skipArgIndex(j, to)
	j = s.skipWhile(j, to, isWidthRune)
	if j < to && s.runes[j] == '.' {
		j = s.skipWhile(j+1, to, isWidthRune)
	}
	j = s.skipArgIndex(j, to)
	if j >= to || !unicode.IsLetter(s.runes[j]) {
		return 0, false
	}
	s.emitCode(i, j+1)
	return j + 1, true
}

func isWidthRune(r rune) bool { return unicode.IsDigit(r) || r == '*' }

func (s *scanner) skipWhile(j, to int, keep func(rune) bool) int {
	for j < to && keep(s.runes[j]) {
		j++
	}
	return j
}

func (s *scanner) skipArgIndex(j, to int) int {
	if j >= to || s.runes[j] != '[' {
		return j
	}
	end := s.skipWhile(j+1, to, unicode.IsDigit)
	if end < to && s.runes[end] == ']' {
		return end + 1
	}
	return j
}
