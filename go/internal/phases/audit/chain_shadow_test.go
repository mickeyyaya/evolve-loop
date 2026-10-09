package audit

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

func TestChainShadow_NeverChangesTheVerdict(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
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
