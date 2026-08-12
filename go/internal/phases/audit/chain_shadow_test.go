package audit

// chain_shadow_test.go — the audit phase consuming the reasoning chain, in
// SHADOW (ADR-0088 rollout).
//
// Shadow means: the chain is parsed, concluded against the evidence the phase
// was actually given, and RECORDED beside the cycle — and the phase's verdict
// is byte-identical to what it would have been without any of it. That is the
// whole point of the stage: a wave produces the comparison data that says
// whether the chain agrees with the narrative verdict, and where it does not,
// WHICH LINK the narrative was silent about. Enforcing before that data exists
// would be the same mistake as every gate this repo has had to walk back.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func reportWithChain(verdict string, c auditchain.Chain) string {
	return "# Audit Report\n\n## Verdict\n**" + verdict + "**\n\n" +
		"## Defects\n\nNone at MEDIUM or above.\n\n" + auditchain.RenderChainBlock(c) + "\n"
}

func fullCoherentChain() auditchain.Chain {
	var c auditchain.Chain
	for _, id := range auditchain.RequiredLinks() {
		c = append(c, auditchain.Link{ID: id, Status: auditchain.StatusCoherent,
			Finding: "holds", Citation: string(id) + "-evidence.md:1"})
	}
	return c
}

// The shadow contract, stated as a test: recording must never move the verdict.
func TestChainShadow_NeverChangesTheVerdict(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	// A chain that CONCLUDES FAIL beside a narrative PASS: the most dangerous
	// shape for a shadow stage to get wrong, because enforcing here would flip
	// a shipping cycle on evidence nobody has soaked yet.
	broken := fullCoherentChain()
	broken[5] = auditchain.Link{ID: auditchain.LinkDelivery, Status: auditchain.StatusIncoherent,
		Finding: "implements a cache; the intent asked for a retry budget", Citation: "intent.md:4"}
	body := reportWithChain("PASS", broken)

	fb := &fakeBridge{writeArtifact: body}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: "/p", Workspace: ws})

	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("shadow moved the verdict: got %s, want PASS — the stage records, it does not decide", resp.Verdict)
	}
}

// And the recording itself: a wave is only useful if the comparison is durable.
func TestChainShadow_RecordsTheConclusionBesideTheCycle(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	broken := fullCoherentChain()
	broken[4] = auditchain.Link{ID: auditchain.LinkNarrative, Status: auditchain.StatusIncoherent,
		Finding: "the report claims a fix the diff does not contain", Citation: "build-report.md:12"}

	fb := &fakeBridge{writeArtifact: reportWithChain("PASS", broken)}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	if _, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: "/p", Workspace: ws}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(ws, auditchain.ShadowRecordFile))
	if err != nil {
		t.Fatalf("no shadow record written — the wave would produce no comparison data: %v", err)
	}
	var rec struct {
		NarrativeVerdict string   `json:"narrative_verdict"`
		ChainVerdict     string   `json:"chain_verdict"`
		Agrees           bool     `json:"agrees"`
		Diagnoses        []string `json:"diagnoses"`
		Rationale        string   `json:"rationale"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("shadow record is not decodable: %v\n%s", err, raw)
	}
	if rec.NarrativeVerdict != "PASS" || rec.ChainVerdict != "FAIL" {
		t.Errorf("record = narrative %q chain %q, want PASS/FAIL", rec.NarrativeVerdict, rec.ChainVerdict)
	}
	if rec.Agrees {
		t.Error("a PASS narrative beside a FAIL chain must be recorded as a DISAGREEMENT — that is the one datum the soak exists to collect")
	}
	if strings.Join(rec.Diagnoses, " ") == "" {
		t.Error("the record must name the human-recognisable pattern (here: specious), or the operator has to re-derive it")
	}
}

// A report with no chain block is the commonest state during rollout (the
// persona change has not reached every driver yet). It must be recorded as
// ABSENT, never inferred as coherent, and never treated as a defect while the
// stage is shadow.
func TestChainShadow_AbsentChainIsRecordedNotInvented(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	fb := &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
	phase := New(Config{Bridge: fb, Prompts: fakePromptsFS("body")})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: "/p", Workspace: ws})
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("an absent chain changed the verdict in shadow: %s", resp.Verdict)
	}
	raw, err := os.ReadFile(filepath.Join(ws, auditchain.ShadowRecordFile))
	if err != nil {
		t.Fatalf("absence must still be recorded — 'no chain' is the measurement: %v", err)
	}
	if !strings.Contains(string(raw), "absent") {
		t.Errorf("the record must say the chain was absent, not report an empty one: %s", raw)
	}
}
