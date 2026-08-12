package auditchain

// chainblock.go — the wire format between the auditor and the pipeline.
//
// The chain has to survive the trip from an LLM's markdown into Go, and this
// repo's history with LLM-authored machine-graded artifacts is unkind: every
// generation that kept the literal example in the prompt and the parser in Go
// drifted apart, and the drift always surfaced as a gate blaming the agent for
// producing what it was told to produce. So the example the auditor is SHOWN
// (ChainBlockExample) is the same constant the tests round-trip through the
// parser — three legs, one shape (ADR-0084 I2).
//
// Format choice: an HTML comment, one row per link, pipe-separated, with the
// finding LAST so a separator inside prose cannot shift the citation out of its
// column. A comment because the block is machine state that must not clutter
// the human report; row-per-link because a missing link has to be visible as a
// missing ROW rather than as a subtly different JSON shape.

import (
	"fmt"
	"strings"
)

const (
	chainBlockOpen  = "<!-- evolve-audit-chain"
	chainBlockClose = "-->"
	chainFieldSep   = " | "
)

// ChainBlockExample is the literal the auditor persona shows. It is a COMPLETE
// chain that concludes PASS, because an auditor copies the shape it is given: a
// partial example teaches a partial chain, and a partial chain is exactly what
// Conclude treats as decisive.
const ChainBlockExample = `<!-- evolve-audit-chain
intent-fidelity | coherent | intent.md:4 | the intent restates the queued item's acceptance criteria without narrowing them
selection-fidelity | coherent | triage-decision.json:12 | top_n names the item the intent describes
specification-fidelity | coherent | covering-tests.md:31 | each acceptance criterion has a test that fails without the change
implementation-fidelity | coherent | build-report.md:58 | the code satisfies those tests as written; no test was relaxed in this diff
narrative-fidelity | coherent | build-report.md:12 | every claim in the report is present in the diff
delivery-fidelity | coherent | intent.md:4 | the change delivers the intent, not an adjacent problem
evidence-fidelity | coherent | acs-verdict.json:1 | the cited gate results were produced by running the gates over these bytes
-->`

// RenderChainBlock writes a chain in the wire format.
func RenderChainBlock(c Chain) string {
	var b strings.Builder
	b.WriteString(chainBlockOpen)
	b.WriteByte('\n')
	for _, l := range c {
		// id | status | citation | finding — the finding is last precisely
		// because it is the only free-prose field.
		b.WriteString(string(l.ID))
		b.WriteString(chainFieldSep)
		b.WriteString(string(l.Status))
		b.WriteString(chainFieldSep)
		b.WriteString(l.Citation)
		b.WriteString(chainFieldSep)
		b.WriteString(strings.ReplaceAll(l.Finding, "\n", " "))
		b.WriteByte('\n')
	}
	b.WriteString(chainBlockClose)
	return b.String()
}

// ParseChainBlock extracts the chain from an audit report.
//
// Every malformed row is an ERROR, never a skip: a dropped row becomes a
// missing link, and Conclude treats a missing link as decisive — so a silent
// skip would turn a formatting slip into a FAIL that names the wrong cause. The
// auditor is told the exact shape; a row that does not match it is a defect
// worth reporting as itself.
func ParseChainBlock(content string) (Chain, error) {
	// LAST block wins — the same tail-anchoring phasecontract's sentinel parser
	// uses, and for the same reason: the persona now carries the delimiter, so
	// an auditor quoting its own instructions above its real block would
	// otherwise have the quote parsed instead (review MEDIUM).
	start := strings.LastIndex(content, chainBlockOpen)
	if start < 0 {
		return nil, fmt.Errorf("auditchain: no chain block in the report — the audit produced a verdict without the reasoning that entails it")
	}
	rest := content[start+len(chainBlockOpen):]
	end := strings.Index(rest, chainBlockClose)
	if end < 0 {
		return nil, fmt.Errorf("auditchain: chain block is not closed with %q", chainBlockClose)
	}
	valid := map[Status]bool{StatusCoherent: true, StatusIncoherent: true, StatusUnverifiable: true}

	var c Chain
	for i, raw := range strings.Split(rest[:end], "\n") {
		line := strings.TrimSpace(raw)
		// A markdown TABLE row is the likeliest wrong shape an auditor emits,
		// and the dangerous one: `| id | status | ... |` splits into exactly
		// four fields, so it parsed into LinkID("| id") and every required link
		// then read as MISSING — a silent, plausible, wrong record rather than
		// a loud absence (review HIGH).
		line = strings.TrimSpace(strings.Trim(line, "|"))
		if line == "" {
			continue
		}
		// SplitN with the finding last: a separator inside the prose stays in
		// the prose instead of shifting every later column.
		parts := strings.SplitN(line, chainFieldSep, 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("auditchain: chain row %d (%q) has %d fields, want 4: id | status | citation | finding", i+1, line, len(parts))
		}
		st := Status(strings.TrimSpace(parts[1]))
		if !valid[st] {
			return nil, fmt.Errorf("auditchain: chain row %d: unknown status %q — the vocabulary is coherent | incoherent | unverifiable, and inventing one hides which of the three it actually was", i+1, st)
		}
		c = append(c, Link{
			ID:       LinkID(strings.TrimSpace(parts[0])),
			Status:   st,
			Citation: strings.TrimSpace(parts[2]),
			Finding:  strings.TrimSpace(parts[3]),
		})
	}
	if len(c) == 0 {
		return nil, fmt.Errorf("auditchain: chain block is empty — an empty chain is not a clean one")
	}
	return c, nil
}
