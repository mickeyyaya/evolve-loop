// Package keyspec validates and classifies tmux key-spec tokens — the body of
// a `keystroke` envelope (the operator's "send any keyevent like a human"
// hatch). It is intentionally non-blocking: Validate only *flags* tokens that
// look like a mistyped key name, so a genuine modal-dismissal keystroke is
// never refused. tmux send-keys accepts arbitrary literal text, so literals
// are always valid; the value-add is catching typos like "Excape" / "Etner"
// before they are sent verbatim and silently do nothing.
//
// Vocabulary mirrors `tmux send-keys`: named keys (Enter, Escape, Tab, arrows,
// PgUp/PgDn, Home/End, F1–F12, …) and modifier combinations (C-c, M-x, S-Tab).
package keyspec

import (
	"regexp"
	"strings"
)

// Class is the classification of a single key token.
type Class int

const (
	// ClassLiteral is plain text/char(s) tmux will type verbatim (e.g. "y").
	ClassLiteral Class = iota
	// ClassNamed is a recognized tmux named key (e.g. "Enter", "Escape").
	ClassNamed
	// ClassModifier is a recognized modifier combination (e.g. "C-c", "M-x").
	ClassModifier
	// ClassSuspect is a token that looks like a key name but is not recognized
	// (e.g. "Excape") — the only class Validate flags.
	ClassSuspect
)

// namedKeys is the recognized tmux key vocabulary (canonical names + the
// common aliases tmux accepts). Lower-cased for case-insensitive lookup.
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

// modifierRE matches one-or-more modifier prefixes (C- M- S-) plus a remainder.
var modifierRE = regexp.MustCompile(`^((?:[CMS]-)+)(.+)$`)

// suspectRE matches a token that LOOKS like a named key — multi-char, starts
// with an uppercase letter, all letters/digits — so an unrecognized one is a
// likely typo rather than intended literal text.
var suspectRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9]+$`)

// Classify categorizes a single, non-empty key token.
func Classify(token string) Class {
	if token == "" {
		return ClassLiteral
	}
	if namedKeys[strings.ToLower(token)] {
		return ClassNamed
	}
	if m := modifierRE.FindStringSubmatch(token); m != nil {
		rest := m[2]
		// C-c (single char) or C-Enter (named remainder) are valid modifiers;
		// a multi-char non-named remainder (C-Excape) is suspect.
		if len([]rune(rest)) == 1 || namedKeys[strings.ToLower(rest)] {
			return ClassModifier
		}
		return ClassSuspect
	}
	if suspectRE.MatchString(token) {
		return ClassSuspect
	}
	return ClassLiteral
}

// Validate splits body into space-separated tokens and returns those that look
// like mistyped key names. An empty result means every token is a recognized
// named key, a valid modifier, or plain literal text. Validate never errors —
// callers WARN on the returned tokens but still send the body (operator hatch).
func Validate(body string) []string {
	var suspect []string
	for _, tok := range strings.Fields(body) {
		if Classify(tok) == ClassSuspect {
			suspect = append(suspect, tok)
		}
	}
	return suspect
}
