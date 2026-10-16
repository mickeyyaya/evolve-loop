// Package keyspec classifies tmux key-spec tokens and flags likely typos in them.
// Validate only flags a token; the caller still sends the keystroke.
package keyspec

import (
	"regexp"
	"strings"
)

type Class int

const (
	ClassLiteral Class = iota
	ClassNamed
	ClassModifier
	ClassSuspect
)

var namedKeys = func() map[string]bool {
	keys := []string{
		"enter", "escape", "esc", "tab", "btab", "space", "bspace", "backspace",
		"up", "down", "left", "right",
		"pgup", "pgdn", "pageup", "pagedown", "ppage", "npage",
		"home", "end", "insert", "ic", "delete", "dc",
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
	}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}()

var modifierRE = regexp.MustCompile(`^((?:[CMS]-)+)(.+)$`)

var suspectRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)

func Classify(token string) Class {
	if token == "" {
		return ClassLiteral
	}
	if namedKeys[strings.ToLower(token)] {
		return ClassNamed
	}
	if m := modifierRE.FindStringSubmatch(token); m != nil {
		rest := m[2]
		isKeyRemainder := len([]rune(rest)) == 1 || namedKeys[strings.ToLower(rest)]
		if isKeyRemainder {
			return ClassModifier
		}
		return ClassSuspect
	}
	if suspectRE.MatchString(token) {
		return ClassSuspect
	}
	return ClassLiteral
}

func Validate(body string) []string {
	var suspect []string
	for _, tok := range strings.Fields(body) {
		if Classify(tok) == ClassSuspect {
			suspect = append(suspect, tok)
		}
	}
	return suspect
}
