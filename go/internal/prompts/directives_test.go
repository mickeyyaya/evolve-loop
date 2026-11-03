package prompts_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

func TestStrippedDirectives_ADirectiveBelowTheMarkerIsReported(t *testing.T) {
	body := "# Persona\n\nRun the scan.\n\n## Reference Index\n\nMANDATORY: write the ledger entry.\n"
	got := prompts.StrippedDirectives(body)
	want := []string{"MANDATORY: write the ledger entry."}
	if !slices.Equal(got, want) {
		t.Errorf("StrippedDirectives = %q, want %q", got, want)
	}
}

func TestStrippedDirectives_ADirectiveAboveTheMarkerOrRepeatedAboveIsKept(t *testing.T) {
	body := "MANDATORY: write the ledger entry.\n\n## Reference Index\n\nMANDATORY: write the ledger entry.\nSee the reference file.\n"
	if got := prompts.StrippedDirectives(body); len(got) != 0 {
		t.Errorf("StrippedDirectives = %q, want none", got)
	}
}

func TestStrippedDirectives_AMarkerlessBodyLosesNothing(t *testing.T) {
	if got := prompts.StrippedDirectives("REQUIRED: read the inbox.\n"); len(got) != 0 {
		t.Errorf("StrippedDirectives = %q, want none", got)
	}
}
