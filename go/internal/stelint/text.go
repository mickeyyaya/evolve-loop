package stelint

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	maxStepWords          = 20
	maxSentenceWords      = 25
	maxParagraphSentences = 6
	excerptPieces         = 12
	closingMarks          = ")]}\"'”’*_"
	openingMarks          = "([{\"'“‘*_"
)

type atom struct {
	r         rune
	code      string
	quotation bool
	line      int
	exempt    bool
}

func (a atom) isCode() bool { return a.code != "" }

type piece []atom

func (p piece) line() int { return p[0].line }

func (p piece) isWord() bool {
	for _, a := range p {
		if a.isCode() || unicode.IsLetter(a.r) || unicode.IsDigit(a.r) {
			return true
		}
	}
	return false
}

func (p piece) text() string {
	var b strings.Builder
	for _, a := range p {
		if a.isCode() {
			b.WriteString(a.code)
			continue
		}
		b.WriteRune(a.r)
	}
	return b.String()
}

func (p piece) technical() bool {
	s := p.text()
	return strings.Contains(s, "://") || strings.Contains(s, "=") || (strings.HasPrefix(s, "[") && strings.Contains(s, "]"))
}

func (p piece) endsSentence(abbrevs map[string]bool) bool {
	end := len(p)
	for end > 0 && !p[end-1].isCode() && strings.ContainsRune(closingMarks, p[end-1].r) {
		end--
	}
	if end == 0 {
		return false
	}
	if last := p[end-1]; last.isCode() {
		return last.quotation && endsWithStop(last.code)
	}
	switch p[end-1].r {
	case '?', '!':
		return true
	case '.':
		return !abbrevs[strings.TrimLeft(strings.ToLower(p[:end].text()), openingMarks)]
	}
	return false
}

func endsWithStop(quotation string) bool {
	inner := []rune(strings.TrimRight(quotation, closingMarks))
	return len(inner) > 0 && strings.ContainsRune(".?!", inner[len(inner)-1])
}

type unit struct {
	text   string
	raw    string
	line   int
	exempt bool
}

func isUnitRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("'’._", r)
}

func (p piece) units() []unit {
	technical := p.technical()
	var out []unit
	var cur []atom
	flush := func() {
		if len(cur) > 0 {
			out = append(out, newUnit(cur, technical))
			cur = nil
		}
	}
	for _, a := range p {
		switch {
		case a.isCode():
			flush()
			out = append(out, unit{line: a.line, exempt: true})
		case isUnitRune(a.r):
			cur = append(cur, a)
		default:
			flush()
		}
	}
	flush()
	return out
}

func newUnit(atoms []atom, technical bool) unit {
	raw := piece(atoms).text()
	u := unit{text: strings.Trim(strings.ToLower(raw), "'’"), raw: raw, line: atoms[0].line, exempt: technical}
	for _, a := range atoms {
		u.exempt = u.exempt || a.exempt
	}
	return u
}

func (u unit) matches(word string) bool {
	if strings.HasSuffix(word, ".") {
		return strings.TrimLeft(u.text, ".") == word
	}
	return strings.Trim(u.text, ".") == word
}

type sentence []piece

func (s sentence) line() int { return s[0].line() }

func (s sentence) wordCount() int {
	n := 0
	for _, p := range s {
		if p.isWord() {
			n++
		}
	}
	return n
}

func (s sentence) excerpt() string {
	parts := make([]string, 0, excerptPieces)
	for i, p := range s {
		if i == excerptPieces {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, p.text())
	}
	return strings.Join(parts, " ")
}

func pieces(atoms []atom) []piece {
	var out []piece
	var cur piece
	for _, a := range atoms {
		if !a.isCode() && unicode.IsSpace(a.r) {
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, a)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func sentences(ps []piece, abbrevs map[string]bool) []sentence {
	var out []sentence
	var cur sentence
	for _, p := range ps {
		cur = append(cur, p)
		if p.endsSentence(abbrevs) {
			out = appendSentence(out, cur)
			cur = nil
		}
	}
	return appendSentence(out, cur)
}

func appendSentence(out []sentence, s sentence) []sentence {
	if s.wordCount() == 0 {
		return out
	}
	return append(out, s)
}

type phrase struct {
	words []string
	sub   Substitution
}

type matcher struct {
	phrases []phrase
	abbrevs map[string]bool
}

func newMatcher(subs []Substitution) matcher {
	m := matcher{abbrevs: map[string]bool{}}
	for _, s := range subs {
		words := strings.Fields(strings.ToLower(s.Phrase))
		if len(words) == 0 {
			continue
		}
		m.phrases = append(m.phrases, phrase{words: words, sub: s})
		if last := words[len(words)-1]; strings.HasSuffix(last, ".") {
			m.abbrevs[last] = true
		}
	}
	return m
}

func (m matcher) sentences(atoms []atom) []sentence {
	return sentences(pieces(atoms), m.abbrevs)
}

func (m matcher) wordFindingsOf(sents []sentence) []Finding {
	var out []Finding
	for _, s := range sents {
		out = append(out, m.wordFindings(s)...)
	}
	return out
}

func lengthFindings(sents []sentence, limit int) []Finding {
	var out []Finding
	for _, s := range sents {
		if n := s.wordCount(); n > limit {
			out = append(out, Finding{Line: s.line(), Rule: RuleSentence, Message: sentenceMessage(n, limit), Excerpt: s.excerpt()})
		}
	}
	return out
}

func sentenceMessage(words, limit int) string {
	if limit == maxStepWords {
		return fmt.Sprintf("a numbered item has %d words; the limit is %d", words, limit)
	}
	return fmt.Sprintf("a sentence has %d words; the limit is %d", words, limit)
}

func (m matcher) wordFindings(s sentence) []Finding {
	var units []unit
	for _, p := range s {
		units = append(units, p.units()...)
	}
	var out []Finding
	for i := range units {
		for _, ph := range m.phrases {
			if matched, ok := phraseAt(units, i, ph.words); ok {
				out = append(out, Finding{
					Line:    units[i].line,
					Rule:    RuleWord,
					Message: fmt.Sprintf("replace %q with %q", ph.sub.Phrase, ph.sub.Replacement),
					Excerpt: matched,
				})
			}
		}
	}
	return out
}

func phraseAt(units []unit, at int, words []string) (string, bool) {
	if at+len(words) > len(units) {
		return "", false
	}
	raw := make([]string, len(words))
	for k, w := range words {
		u := units[at+k]
		if u.exempt || !u.matches(w) {
			return "", false
		}
		raw[k] = u.raw
	}
	return strings.Join(raw, " "), true
}

func paragraphFinding(sents []sentence) (Finding, bool) {
	if len(sents) <= maxParagraphSentences {
		return Finding{}, false
	}
	return Finding{
		Line:    sents[0].line(),
		Rule:    RuleParagraph,
		Message: fmt.Sprintf("a paragraph has %d sentences; the limit is %d", len(sents), maxParagraphSentences),
		Excerpt: sents[0].excerpt(),
	}, true
}

func generatedNotice(line int, what string) Finding {
	return Finding{Line: line, Rule: RuleSkippedGenerated, Message: "a generator writes " + what + "; the lint does not check it"}
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}
