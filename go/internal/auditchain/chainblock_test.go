package auditchain

// chainblock_test.go — the wire format, and the three-legged single-sourcing
// that keeps it honest (ADR-0084 I2): the literal example the PERSONA shows the
// auditor, the Go PARSER that reads what comes back, and a REAL round trip must
// all be the same shape. Every prior generation of this repo's LLM-authored
// artifacts drifted the moment those three lived in different files.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The load-bearing test: the example an auditor is shown must parse, and must
// parse into a chain that concludes. An example that cannot round-trip is an
// instruction to produce something the parser will reject.
func TestChainBlockExample_IsParseableAndComplete(t *testing.T) {
	t.Parallel()
	c, err := ParseChainBlock(ChainBlockExample)
	if err != nil {
		t.Fatalf("the literal example shown to the auditor does not parse: %v\n%s", err, ChainBlockExample)
	}
	if errs := Validate(c); len(errs) != 0 {
		t.Fatalf("the example is not a valid chain: %v", errs)
	}
	if len(c) != len(RequiredLinks()) {
		t.Errorf("the example must demonstrate EVERY required link (an auditor copies what it is shown); got %d of %d", len(c), len(RequiredLinks()))
	}
	if got := Conclude(c); got.Verdict != VerdictPASS {
		t.Errorf("the example must conclude cleanly so its shape is unambiguous; got %s (%s)", got.Verdict, got.Rationale)
	}
}

func TestParseChainBlock_ReadsAReportsEmbeddedChain(t *testing.T) {
	t.Parallel()
	report := "# Audit Report\n\n## Verdict\n**FAIL**\n\nprose prose\n\n" + ChainBlockExample + "\n\nmore prose\n"
	c, err := ParseChainBlock(report)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c) != len(RequiredLinks()) {
		t.Errorf("got %d links from an embedded block, want %d", len(c), len(RequiredLinks()))
	}
}

// Absence is its own answer, and a distinguishable one: "the auditor emitted no
// chain" must never look like "the auditor emitted an empty one".
func TestParseChainBlock_AbsenceIsNamed(t *testing.T) {
	t.Parallel()
	if _, err := ParseChainBlock("# Audit Report\n\n## Verdict\n**PASS**\n"); err == nil {
		t.Error("a report with no chain block parsed silently — the absence must be reportable")
	} else if !strings.Contains(err.Error(), "no chain block") {
		t.Errorf("the error must name the absence; got %v", err)
	}
}

func TestParseChainBlock_RejectsMalformedRowsLoudly(t *testing.T) {
	t.Parallel()
	bad := "<!-- evolve-audit-chain\n" +
		"intent-fidelity | coherent | finding | citation.md:1\n" +
		"this row has too few fields\n" +
		"-->\n"
	if _, err := ParseChainBlock(bad); err == nil {
		t.Error("a malformed row was skipped silently — a dropped link becomes a missing link, which the conclusion treats as decisive; it must not happen quietly")
	}

	unknownStatus := "<!-- evolve-audit-chain\n" +
		"intent-fidelity | probably-fine | finding | citation.md:1\n-->\n"
	if _, err := ParseChainBlock(unknownStatus); err == nil {
		t.Error("an invented status was accepted — the vocabulary is the contract")
	}
}

// Round trip: what Render writes, Parse reads back identically. This is what
// lets the shadow comparison be trusted — a difference in the record must mean
// a difference in the reasoning, never a difference in the formatting.
func TestRenderChainBlock_RoundTrips(t *testing.T) {
	t.Parallel()
	orig := setLink(fullChain(), LinkNarrative, StatusIncoherent, "report claims a retry budget the diff does not implement")
	got, err := ParseChainBlock(RenderChainBlock(orig))
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if len(got) != len(orig) {
		t.Fatalf("round trip lost links: %d vs %d", len(got), len(orig))
	}
	for i := range got {
		if got[i] != orig[i] {
			t.Errorf("link %d changed across the round trip:\n have %+v\n want %+v", i, got[i], orig[i])
		}
	}
}

// A finding containing the field separator must not silently split into a new
// column and shift the citation out of place.
func TestChainBlock_FindingMayContainTheSeparator(t *testing.T) {
	t.Parallel()
	c := setLink(fullChain(), LinkDelivery, StatusIncoherent, "asked for A | delivered B")
	got, err := ParseChainBlock(RenderChainBlock(c))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, l := range got {
		if l.ID == LinkDelivery {
			if l.Finding != "asked for A | delivered B" {
				t.Errorf("finding mangled by the separator: %q", l.Finding)
			}
			if l.Citation == "" {
				t.Error("citation was shifted out of its column by a separator inside the finding")
			}
		}
	}
}

// TestPersona_InstructsExactlyTheShapeTheParserReads is the third leg of the
// single-sourcing (ADR-0084 I2): the persona the auditor is DISPATCHED with,
// the parser, and the example must agree. Every prior generation of an
// LLM-authored graded artifact in this repo drifted the moment these lived in
// separate files, and the drift always surfaced as a gate blaming the agent for
// producing what it had been told to produce.
func TestPersona_InstructsExactlyTheShapeTheParserReads(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "agents", "evolve-auditor.md"))
	if err != nil {
		t.Skipf("auditor persona not readable from here: %v", err)
	}
	persona := string(raw)

	// Every link the parser requires must be named to the auditor, or it is
	// being graded on a chain nobody asked it for.
	for _, id := range RequiredLinks() {
		if !strings.Contains(persona, string(id)) {
			t.Errorf("the persona never names link %q, but Conclude treats its absence as decisive", id)
		}
	}
	// The delimiters and the status vocabulary must match the parser's.
	for _, tok := range []string{chainBlockOpen, chainBlockClose,
		string(StatusCoherent), string(StatusIncoherent), string(StatusUnverifiable)} {
		if !strings.Contains(persona, tok) {
			t.Errorf("the persona does not carry %q — the auditor cannot emit a shape it was never shown", tok)
		}
	}
	// And the instruction must survive compaction: operational directives live
	// ABOVE the strip marker (cycle-1390–1429 lesson — a mid-file marker
	// deleted the verdict rules from every dispatched audit).
	chainAt := strings.Index(persona, chainBlockOpen)
	markerAt := strings.Index(persona, "## Reference Index")
	if markerAt >= 0 && chainAt > markerAt {
		t.Error("the chain instruction sits BELOW the strip marker — it would be deleted from every dispatched prompt, which is exactly how the auditor lost its verdict rules for 40 cycles")
	}
}

// The two shapes the review showed produce a WRONG record rather than a loud
// absence — the dangerous failure mode for soak data.
func TestParseChainBlock_HandlesTheShapesAnLLMActuallyEmits(t *testing.T) {
	t.Parallel()
	// A markdown TABLE row: splits into four fields and used to yield
	// LinkID("| intent-fidelity"), so every required link read as missing and
	// the record said "incomplete chain" about a complete one.
	table := "<!-- evolve-audit-chain\n"
	for _, id := range RequiredLinks() {
		table += "| " + string(id) + " | coherent | evidence.md:1 | holds |\n"
	}
	table += "-->\n"
	c, err := ParseChainBlock(table)
	if err != nil {
		t.Fatalf("a table-shaped chain must parse, not mis-parse: %v", err)
	}
	if got := Conclude(c); got.Verdict != VerdictPASS {
		t.Errorf("a complete table-shaped chain concluded %s (%s) — the pipes were read as part of the link id", got.Verdict, got.Rationale)
	}

	// An auditor that quotes its own instructions above its real block: the
	// persona now carries the delimiter, so first-block-wins would parse the
	// echo instead of the verdict's actual reasoning.
	echoed := "Per my instructions I must emit:\n" + RenderChainBlock(fullChain()) +
		"\n\nMy actual reasoning:\n" +
		RenderChainBlock(setLink(fullChain(), LinkDelivery, StatusIncoherent, "delivers a cache; the intent asked for a retry budget"))
	got, err := ParseChainBlock(echoed)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if Conclude(got).Verdict != VerdictFAIL {
		t.Error("the ECHOED example was parsed instead of the report's own block — the auditor's real finding was discarded")
	}
}
