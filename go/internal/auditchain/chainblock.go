package auditchain

import (
	"fmt"
	"strings"
)

const (
	chainBlockOpen  = "<!-- evolve-audit-chain"
	chainBlockClose = "-->"
	chainFieldSep   = " | "
)

const ChainBlockExample = `<!-- evolve-audit-chain
intent-fidelity | coherent | intent.md:4 | the intent restates the queued item's acceptance criteria without narrowing them
selection-fidelity | coherent | triage-decision.json:12 | top_n names the item the intent describes
specification-fidelity | coherent | covering-tests.md:31 | each acceptance criterion has a test that fails without the change
implementation-fidelity | coherent | build-report.md:58 | the code satisfies those tests as written; no test was relaxed in this diff
narrative-fidelity | coherent | build-report.md:12 | every claim in the report is present in the diff
delivery-fidelity | coherent | intent.md:4 | the change delivers the intent, not an adjacent problem
evidence-fidelity | coherent | acs-verdict.json:1 | the cited gate results were produced by running the gates over these bytes
-->`

func RenderChainBlock(c Chain) string {
	var b strings.Builder
	b.WriteString(chainBlockOpen)
	b.WriteByte('\n')
	for _, l := range c {
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

func ParseChainBlock(content string) (Chain, error) {
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
		line = strings.TrimSpace(strings.Trim(line, "|"))
		if line == "" {
			continue
		}
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
