package audit

// The auditor must be shown the reasoning-chain shape, not only told about it.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_ShowsTheAuditorTheChainShape(t *testing.T) {
	t.Parallel()
	got := hooks{}.ComposePrompt("PERSONA BODY", core.PhaseRequest{
		Cycle: 1450, ProjectRoot: "/p", Workspace: t.TempDir(),
	})

	if !strings.Contains(got, auditchain.ChainBlockExample) {
		t.Fatal("the dispatched prompt does not contain the literal example — the auditor is told a format it is never shown, which is the measured cause of 1-in-3 compliance")
	}
	// The example must be parseable AS DISPATCHED: an example that the parser
	// would reject is an instruction to produce something unreadable.
	c, err := auditchain.ParseChainBlock(got)
	if err != nil {
		t.Fatalf("the example inside the real prompt does not parse: %v", err)
	}
	if len(c) != len(auditchain.RequiredLinks()) {
		t.Errorf("the dispatched example demonstrates %d links, want all %d — an auditor copies what it is shown, and a partial example teaches a partial chain", len(c), len(auditchain.RequiredLinks()))
	}
	// And the pre-existing prompt content must survive.
	if !strings.Contains(got, "PERSONA BODY") {
		t.Error("the persona body was lost")
	}
}

// The example is illustration; the auditor's OWN block is the verdict's
// reasoning. Since the prompt now contains a fully-coherent example, an auditor
// that echoes its instructions must not have that echo read as its findings —
// the parser is tail-anchored for exactly this reason, and this pins the
// property end-to-end from the real composed prompt.
func TestComposePrompt_ExampleCannotBeMistakenForTheAuditorsOwnChain(t *testing.T) {
	t.Parallel()
	prompt := hooks{}.ComposePrompt("PERSONA BODY", core.PhaseRequest{
		Cycle: 1450, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	// An auditor that quotes the whole prompt back and then reports a real,
	// incoherent finding of its own.
	report := "# Audit Report\n\n" + prompt + "\n\n## Verdict\n**FAIL**\n\n" +
		auditchain.RenderChainBlock(auditchain.Chain{
			{ID: auditchain.LinkIntentFidelity, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "intent.md:1"},
			{ID: auditchain.LinkSelection, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "triage-decision.json:1"},
			{ID: auditchain.LinkSpecification, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "covering-tests.md:1"},
			{ID: auditchain.LinkImplementation, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "build-report.md:1"},
			{ID: auditchain.LinkNarrative, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "build-report.md:2"},
			{ID: auditchain.LinkDelivery, Status: auditchain.StatusIncoherent, Finding: "delivers a cache; the intent asked for a retry budget", Citation: "intent.md:4"},
			{ID: auditchain.LinkEvidence, Status: auditchain.StatusCoherent, Finding: "ok", Citation: "acs-verdict.json:1"},
		})

	c, err := auditchain.ParseChainBlock(report)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := auditchain.Conclude(c); got.Verdict != auditchain.VerdictFAIL {
		t.Errorf("the PROMPT's coherent example was read as the auditor's reasoning (%s) — its real finding was discarded", got.Verdict)
	}
}
