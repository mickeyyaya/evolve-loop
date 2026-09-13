package log

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// SanitizeField is the unquoted twin of DiagnosticField: the same UTF-8 repair
// and rune cap (ONE rule, shared), with control characters and line separators
// replaced by a space instead of escaped — for values that land inside a JSON
// string or a single-line log record (the Signal Center's fields, ADR-0101).
func TestSanitizeField_SharesTheBoundWithDiagnosticFieldAndStripsControls(t *testing.T) {
	t.Parallel()
	in := "line\nbreak\ttab\x1b[31mred sep\xffbad"
	got := SanitizeField(in)
	if strings.ContainsAny(got, "\n\t\x1b ") || !utf8.ValidString(got) {
		t.Errorf("no raw control/line-separator bytes, valid UTF-8: %q", got)
	}
	if !strings.Contains(got, "line break") || !strings.Contains(got, "�") {
		t.Errorf("controls become a single space and invalid bytes the replacement rune: %q", got)
	}
	long := strings.Repeat("é", 2000)
	capped := SanitizeField(long)
	if utf8.RuneCountInString(capped) != maxDiagnosticRunes || !strings.HasSuffix(capped, "…") {
		t.Errorf("rune-capped at %d with an ellipsis, got %d runes", maxDiagnosticRunes, utf8.RuneCountInString(capped))
	}
	// The quoted twin bounds identically: DiagnosticField(x) == QuoteToASCII(bounded x).
	if DiagnosticField(long) != strconv.QuoteToASCII(capped) {
		t.Error("DiagnosticField and SanitizeField must share the same bound")
	}
	if SanitizeField("plain value") != "plain value" {
		t.Error("clean input is returned unchanged")
	}
}

func TestSanitizeField_FoldsBidiOverridesAndOtherFormatCharacters(t *testing.T) {
	t.Parallel()
	in := "collector offline\n[bridge] forged\u202e\u200b\ufeff"
	got := SanitizeField(in)
	for _, r := range []rune{'\u202e', '\u200b', '\ufeff', '\n'} {
		if strings.ContainsRune(got, r) {
			t.Errorf("format character %U must not survive: %q", r, got)
		}
	}
	if got != "collector offline [bridge] forged   " {
		t.Errorf("each unsafe rune becomes one space: %q", got)
	}
}
