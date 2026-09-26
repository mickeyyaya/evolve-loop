package triagecap

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagedecision"
)

// The parser's top_n heading and phasecontract.Triage's canonical section are one belief.
func TestTopNHeadingIsTheContracts(t *testing.T) {
	if len(phasecontract.Triage.Sections) == 0 {
		t.Fatal("phasecontract.Triage has no sections")
	}
	canonical := phasecontract.Triage.Sections[0].Canonical
	if _, ok := triagedecision.SectionBody(canonical+"\n- item-a: do it\n", "top_n"); !ok {
		t.Fatalf("the parser does not read the contract's heading %q", canonical)
	}
}
