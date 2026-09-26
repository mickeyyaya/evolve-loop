package guards

import (
	"strings"
)

// shellCommand is one simple command the shell runs: its text for verb matching and its words, unquoted,
// for naming what it executes.
type shellCommand struct {
	text  string
	words []string
}

// splitShellCommands splits a Bash tool command into the simple commands the shell would run.
// Commands end at ; & | newline and parentheses. A command substitution, $( ) or backticks, is a
// command of its own even inside double quotes, because the shell runs it. Comments and heredoc
// bodies are dropped. Only an unquoted << outside a comment and outside parentheses opens a heredoc,
// since inside them << may be an arithmetic shift, as in $((x<<y)).
// It is a guard's approximation, not a parser: a close it cannot place ends a heredoc early, which
// shows the guard more text, never less.
func splitShellCommands(cmd string) []shellCommand {
	s := &shellScanner{src: cmd}
	s.push(0)
	for s.i < len(s.src) {
		if c := s.top(); c.inDquote {
			s.scanDquoted(c)
		} else {
			s.scanUnquoted(c)
		}
	}
	for len(s.stack) > 0 {
		s.pop()
	}
	return s.out
}

type shellScanner struct {
	src      string
	i        int
	stack    []*scanContext
	heredocs []string
	out      []shellCommand
}

// scanContext is the top level or one open command substitution, closed by closer.
type scanContext struct {
	closer   byte
	depth    int
	inDquote bool
	cmd      commandBuilder
}

func (s *shellScanner) top() *scanContext { return s.stack[len(s.stack)-1] }

func (s *shellScanner) push(closer byte) {
	s.stack = append(s.stack, &scanContext{closer: closer})
}

func (s *shellScanner) pop() {
	s.endCommand(s.top())
	s.stack = s.stack[:len(s.stack)-1]
}

func (s *shellScanner) peek(n int) byte {
	if s.i+n < len(s.src) {
		return s.src[s.i+n]
	}
	return 0
}

func (s *shellScanner) endCommand(c *scanContext) {
	if cmd, ok := c.cmd.finish(); ok {
		s.out = append(s.out, cmd)
	}
}

func (s *shellScanner) scanUnquoted(c *scanContext) {
	ch := s.src[s.i]
	switch {
	case ch == '\n':
		s.endCommand(c)
		s.i++
		s.skipHeredocBodies()
	case ch == ';' || ch == '|' || ch == '&':
		s.endCommand(c)
		s.i++
	case ch == ' ' || ch == '\t':
		c.cmd.space(ch)
		s.i++
	case ch == '#' && !c.cmd.inWord:
		s.skipComment()
	case ch == '\'':
		s.singleQuoted(c)
	case ch == '"':
		c.inDquote = true
		c.cmd.quote('"')
		s.i++
	case ch == '\\':
		s.escaped(c)
	case ch == '`' && c.closer == '`':
		s.i++
		s.pop()
	case ch == '`' || (ch == '$' && s.peek(1) == '('):
		s.openSubstitution(c)
	case ch == '(' || ch == ')':
		s.parenthesis(c, ch)
	case ch == '<' && c.depth == 0 && strings.HasPrefix(s.src[s.i:], "<<"):
		s.heredocOrHereString(c)
	default:
		c.cmd.char(ch)
		s.i++
	}
}

func (s *shellScanner) scanDquoted(c *scanContext) {
	switch ch := s.src[s.i]; {
	case ch == '"':
		c.inDquote = false
		c.cmd.quote('"')
		s.i++
	case ch == '\\':
		s.escaped(c)
	case ch == '`' || (ch == '$' && s.peek(1) == '('):
		s.openSubstitution(c)
	default:
		c.cmd.char(ch)
		s.i++
	}
}

func (s *shellScanner) openSubstitution(c *scanContext) {
	closer := byte(')')
	if s.src[s.i] == '`' {
		closer = '`'
		s.i++
	} else {
		s.i += 2
	}
	c.cmd.char('$')
	s.push(closer)
}

func (s *shellScanner) parenthesis(c *scanContext, ch byte) {
	s.i++
	s.endCommand(c)
	switch {
	case ch == '(':
		c.depth++
	case c.depth > 0:
		c.depth--
	case c.closer == ')':
		s.stack = s.stack[:len(s.stack)-1]
	}
}

func (s *shellScanner) singleQuoted(c *scanContext) {
	end := strings.IndexByte(s.src[s.i+1:], '\'')
	if end < 0 {
		end = len(s.src) - s.i - 1
	}
	body := s.src[s.i+1 : s.i+1+end]
	c.cmd.quote('\'')
	c.cmd.literal(body)
	c.cmd.quote('\'')
	s.i += end + 2
}

func (s *shellScanner) escaped(c *scanContext) {
	if s.peek(1) == '\n' {
		s.i += 2
		return
	}
	c.cmd.text.WriteByte('\\')
	s.i++
	if s.i < len(s.src) {
		c.cmd.char(s.src[s.i])
		s.i++
	}
}

func (s *shellScanner) skipComment() {
	if end := strings.IndexByte(s.src[s.i:], '\n'); end >= 0 {
		s.i += end
		return
	}
	s.i = len(s.src)
}

func (s *shellScanner) heredocOrHereString(c *scanContext) {
	rest := s.src[s.i:]
	if strings.HasPrefix(rest, "<<<") {
		c.cmd.literal("<<<")
		s.i += 3
		return
	}
	n := len("<<")
	if strings.HasPrefix(rest[n:], "-") {
		n++
	}
	for n < len(rest) && (rest[n] == ' ' || rest[n] == '\t') {
		n++
	}
	width, marker := heredocWord(rest[n:])
	if marker != "" {
		s.heredocs = append(s.heredocs, marker)
	}
	c.cmd.literal(rest[:n+width])
	s.i += n + width
}

// heredocWord reads the heredoc delimiter word at the start of src and returns its width and its
// quote-removed text, which is the line bash waits for: the whole word, not an identifier prefix.
func heredocWord(src string) (int, string) {
	var marker strings.Builder
	i := 0
	for i < len(src) && !strings.ContainsRune(" \t\n;&|()<>", rune(src[i])) {
		switch src[i] {
		case '\'':
			end := strings.IndexByte(src[i+1:], '\'')
			if end < 0 {
				end = len(src) - i - 1
			}
			marker.WriteString(src[i+1 : i+1+end])
			i += end + 2
		case '"':
			i += dquotedWordPart(src[i+1:], &marker) + 1
		case '\\':
			if i+1 < len(src) && src[i+1] != '\n' {
				marker.WriteByte(src[i+1])
			}
			i += 2
		default:
			marker.WriteByte(src[i])
			i++
		}
	}
	return min(i, len(src)), marker.String()
}

// dquotedWordPart appends the quote-removed text of a double-quoted part to marker and returns the
// width consumed, through the closing quote. A backslash escapes only $ ` " \ and newline, as in bash.
func dquotedWordPart(src string, marker *strings.Builder) int {
	i := 0
	for i < len(src) && src[i] != '"' {
		if src[i] == '\\' && i+1 < len(src) && strings.ContainsRune("$`\"\\\n", rune(src[i+1])) {
			if src[i+1] != '\n' {
				marker.WriteByte(src[i+1])
			}
			i += 2
			continue
		}
		marker.WriteByte(src[i])
		i++
	}
	return i + 1
}

// skipHeredocBodies drops the bodies of the heredocs opened on the line just ended, in order. A body
// ends at a line holding its marker after leading blanks, or the marker then ")"; the scan resumes
// right after the marker. An unterminated body runs to the end of the input, as in bash.
func (s *shellScanner) skipHeredocBodies() {
	for _, marker := range s.heredocs {
		for s.i < len(s.src) {
			line := s.src[s.i:]
			if end := strings.IndexByte(line, '\n'); end >= 0 {
				line = line[:end]
			}
			trimmed := strings.TrimLeft(line, " \t")
			if trimmed == marker || strings.HasPrefix(trimmed, marker+")") {
				s.i += len(line) - len(trimmed) + len(marker)
				break
			}
			s.i += len(line) + 1
		}
	}
	if s.i > len(s.src) {
		s.i = len(s.src)
	}
	s.heredocs = nil
}

// commandBuilder accumulates one command's text and words.
type commandBuilder struct {
	text   strings.Builder
	word   strings.Builder
	words  []string
	inWord bool
}

func (b *commandBuilder) char(ch byte) {
	b.text.WriteByte(ch)
	b.word.WriteByte(ch)
	b.inWord = true
}

func (b *commandBuilder) literal(s string) {
	b.text.WriteString(s)
	b.word.WriteString(s)
	b.inWord = true
}

func (b *commandBuilder) quote(q byte) {
	b.text.WriteByte(q)
	b.inWord = true
}

func (b *commandBuilder) space(ch byte) {
	b.endWord()
	b.text.WriteByte(ch)
}

func (b *commandBuilder) endWord() {
	if b.inWord {
		b.words = append(b.words, b.word.String())
	}
	b.word.Reset()
	b.inWord = false
}

func (b *commandBuilder) finish() (shellCommand, bool) {
	b.endWord()
	cmd := shellCommand{text: b.text.String(), words: b.words}
	*b = commandBuilder{}
	return cmd, strings.TrimSpace(cmd.text) != ""
}
