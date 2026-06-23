package panetrust_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/panetrust"
)

// TestRedactSecrets_NamesExportedWrapper exercises the WS3-S1 exported redaction
// entry point (ADR-0052): a secret-shaped token is replaced with the marker and
// clean text is returned unchanged. It names panetrust.RedactSecrets for the
// public-API coverage gate; the redaction policy is proven by the Digest suite,
// so this pins that the wrapper delegates to the same core.
func TestRedactSecrets_NamesExportedWrapper(t *testing.T) {
	t.Parallel()
	if got := panetrust.RedactSecrets("key sk-livesecret0123456789ABCDEF end"); strings.Contains(got, "sk-livesecret") || !strings.Contains(got, "[REDACTED]") {
		t.Errorf("RedactSecrets must redact a secret-shaped token, got %q", got)
	}
	if got := panetrust.RedactSecrets("no secrets here"); got != "no secrets here" {
		t.Errorf("RedactSecrets must leave clean text unchanged, got %q", got)
	}
}

// TestExtractKind_QuestionConstant names the ExtractKind type and pins the only
// allowlisted kind's wire value. The string value is load-bearing: it is the
// stable name of the one extraction the privileged path accepts.
func TestExtractKind_QuestionConstant(t *testing.T) {
	t.Parallel()
	var k panetrust.ExtractKind = panetrust.ExtractQuestion
	if string(k) != "question" {
		t.Errorf("ExtractQuestion = %q, want %q", k, "question")
	}
}

// TestExtraction_FullValueOnQuestion names the Extraction struct and pins, via
// full-struct equality, exactly what Extract returns for a clean trailing
// question: Kind=ExtractQuestion and the neutralized question line as Value.
// This is stronger than the existing field-by-field substring checks — it
// proves there are no other/unexpected fields populated.
func TestExtraction_FullValueOnQuestion(t *testing.T) {
	t.Parallel()
	pane := "some output\nWhich directory should I use?\n"
	got, err := panetrust.Extract(pane, panetrust.ExtractSpec{Kind: panetrust.ExtractQuestion})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := panetrust.Extraction{
		Kind:  panetrust.ExtractQuestion,
		Value: "Which directory should I use?",
	}
	if got != want {
		t.Errorf("Extraction = %+v, want %+v", got, want)
	}
}
