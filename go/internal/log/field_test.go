package log

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiagnosticField_QuotesAndBoundsUntrustedText(t *testing.T) {
	value := "collector offline\n[bridge] forged\u202e" + strings.Repeat("x", 600)
	got := DiagnosticField(value)

	if strings.ContainsRune(got, '\n') || strings.ContainsRune(got, '\u202e') {
		t.Fatalf("DiagnosticField returned raw line or formatting control: %q", got)
	}
	if !strings.HasPrefix(got, `"collector offline\n[bridge] forged\u202e`) {
		t.Fatalf("DiagnosticField did not preserve escaped evidence: %q", got)
	}
	if !strings.Contains(got, "\\u2026") {
		t.Fatalf("DiagnosticField did not mark bounded output: %q", got)
	}
	unquoted, err := strconv.Unquote(got)
	if err != nil {
		t.Fatalf("DiagnosticField returned invalid quoted text: %v", err)
	}
	if runes := utf8.RuneCountInString(unquoted); runes > 512 {
		t.Fatalf("DiagnosticField returned %d runes, want at most 512", runes)
	}
}
