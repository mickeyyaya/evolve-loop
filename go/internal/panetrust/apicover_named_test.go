package panetrust_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
)

func TestRedactSecrets_NamesExportedWrapper(t *testing.T) {
	t.Parallel()
	if got := panetrust.RedactSecrets("key sk-livesecret0123456789ABCDEF end"); strings.Contains(got, "sk-livesecret") || !strings.Contains(got, "[REDACTED]") {
		t.Errorf("RedactSecrets must redact a secret-shaped token, got %q", got)
	}
	if got := panetrust.RedactSecrets("no secrets here"); got != "no secrets here" {
		t.Errorf("RedactSecrets must leave clean text unchanged, got %q", got)
	}
}

func TestExtractKind_QuestionConstant(t *testing.T) {
	t.Parallel()
	var k panetrust.ExtractKind = panetrust.ExtractQuestion
	if string(k) != "question" {
		t.Errorf("ExtractQuestion = %q, want %q", k, "question")
	}
}

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
