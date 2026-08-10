package bridge

// bridge_contract_minted_test.go — pins the cycle-1424 infra-systemic halt
// class: a MINTED phase (agent name unknown to the contract resolver) was
// dispatched with a naked 2.1KB prompt — no "## Deliverable Contract" block,
// no "DELIVERABLE PATH:" footer — while the engine polled for its report
// artifact. The agent was never told the path; 600s artifact-timeout, exit 81,
// SYSTEM-FAILURE HALT. Rule: whenever the request carries an ArtifactPath the
// engine will poll, the prompt MUST name that path, contract resolution hit or
// miss. Non-phase bridge callers (no ArtifactPath) stay byte-identical.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaunch_UnresolvedAgentWithArtifactStillGetsPathFooter(t *testing.T) {
	fe := &fakeEngine{}
	artifact := "/abs/.evolve/runs/cycle-1424/defect-disposition-ledger-report.md"
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "MINTED-BODY",
		Workspace: t.TempDir(), ArtifactPath: artifact, Agent: "defect-disposition-ledger",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	if !strings.Contains(got, artifact) {
		t.Fatalf("a minted/unresolved agent with a polled ArtifactPath must be TOLD the path in the prompt text (cycle-1424: 600s timeout writing nothing):\n%s", truncate(got, 400))
	}
	if !strings.Contains(got, "DELIVERABLE PATH:") {
		t.Errorf("the footer marker line must be present for tooling greps")
	}
	if i := strings.Index(got, "MINTED-BODY"); strings.Index(got, artifact) < i {
		t.Errorf("the synthesized path footer must land AFTER the body (generation point)")
	}
	// Negative pins (adversarial-review): the synthesized disclosure must not
	// smuggle instructions the agent cannot satisfy — no self-check line
	// (`evolve phase verify <miss-agent>` is guaranteed exit 10) and no
	// verdict-sentinel template (nothing registered any verdict vocabulary).
	if strings.Contains(got, "phase verify") {
		t.Errorf("synthesized footer must not instruct an impossible self-check for a resolver-miss agent:\n%s", truncate(got, 400))
	}
	if strings.Contains(got, "<verdict-sentinel") || strings.Contains(got, "<self-check>") {
		t.Errorf("synthesized footer must carry no sentinel/self-check blocks:\n%s", truncate(got, 400))
	}
}

// (No "unresolved agent without artifact" case exists at this seam: Launch
// validation rejects an empty ArtifactPath outright — "bridge: ArtifactPath
// required" — so every dispatched prompt has a pollable path to disclose. The
// empty-path guard inside injectContract covers direct non-Launch callers.)
