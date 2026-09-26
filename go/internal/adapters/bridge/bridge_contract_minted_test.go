package bridge

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
	if strings.Contains(got, "phase verify") {
		t.Errorf("synthesized footer must not instruct an impossible self-check for a resolver-miss agent:\n%s", truncate(got, 400))
	}
	if strings.Contains(got, "<verdict-sentinel") || strings.Contains(got, "<self-check>") {
		t.Errorf("synthesized footer must carry no sentinel/self-check blocks:\n%s", truncate(got, 400))
	}
}
