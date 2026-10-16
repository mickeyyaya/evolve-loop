package logfilter

import (
	"fmt"
	"strings"
	"unicode"
)

type plainTextState struct {
	prev  string
	count int
	have  bool
}

func newPlainTextState() *plainTextState { return &plainTextState{} }

func (p *plainTextState) next(line string) (string, bool) {
	if isNoise(line) {
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

func isSpinnerRune(r rune) bool {
	switch r {
	case '⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏':
		return true
	case '|', '/', '-', '\\':
		return true
	}
	return false
}

func isBorderRune(r rune) bool {
	switch r {
	case '╭', '╮', '╰', '╯', '─', '│', '┌', '┐', '└', '┘', '═', '║':
		return true
	}
	return false
}
