package logfilter

import (
	"fmt"
	"strings"
	"unicode"
)

// plainTextState tracks running deduplication of consecutive identical
// lines so we can collapse spinner-style redraws and noisy hook
// repetitions into a single "(× N times)" marker.
type plainTextState struct {
	prev  string
	count int
	have  bool
}

func newPlainTextState() *plainTextState { return &plainTextState{} }

// next consumes one plain-text line. Returns (formatted, emit=true) when
// the caller should write a line; (_, false) means the line was dropped
// (noise) OR was absorbed into a pending dedup run.
func (p *plainTextState) next(line string) (string, bool) {
	if isNoise(line) {
		// Flush any pending dedup run, since noise breaks the streak.
		flushed := p.flush()
		if flushed != "" {
			return flushed, true
		}
		return "", false
	}
	if p.have && line == p.prev {
		p.count++
		return "", false
	}
	flushed := p.flush()
	p.prev = line
	p.count = 1
	p.have = true
	if flushed != "" {
		return flushed, true
	}
	return "", false
}

// flush returns any pending dedup run as a string ("" if nothing pending).
// Called at end-of-stream and on JSON-line transitions.
func (p *plainTextState) flush() string {
	if !p.have {
		return ""
	}
	out := p.prev
	if p.count > 1 {
		out = fmt.Sprintf("%s (× %d times)", p.prev, p.count)
	}
	p.have = false
	p.count = 0
	p.prev = ""
	return out + "\n"
}

// isNoise reports lines that carry no signal:
//   - completely blank (whitespace only),
//   - just spinner / progress glyphs,
//   - just box-drawing borders with no inner content.
func isNoise(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	allSpinner := true
	allBorder := true
	for _, r := range trimmed {
		if !isSpinnerRune(r) && !unicode.IsSpace(r) {
			allSpinner = false
		}
		if !isBorderRune(r) && !unicode.IsSpace(r) {
			allBorder = false
		}
		if !allSpinner && !allBorder {
			return false
		}
	}
	return allSpinner || allBorder
}

// isSpinnerRune covers the braille-pattern spinner glyphs Claude Code's
// TUI uses, plus a few common ASCII rotators.
func isSpinnerRune(r rune) bool {
	switch r {
	case '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏':
		return true
	case '|', '/', '-', '\\':
		return true
	}
	return false
}

// isBorderRune covers the Unicode box-drawing characters used by
// Claude Code's bordered panels.
func isBorderRune(r rune) bool {
	switch r {
	case '╭', '╮', '╰', '╯', '─', '│', '┌', '┐', '└', '┘', '═', '║':
		return true
	}
	return false
}
