package panetrust_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
)

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
