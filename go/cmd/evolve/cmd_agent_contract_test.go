package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type composingBridge struct {
	real     core.Bridge
	resolved string
}

func (b *composingBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	composed, err := b.real.(*bridge.Adapter).ComposePrompt(req)
	if err != nil {
		return core.BridgeResponse{}, err
	}
	b.resolved = composed.Prompt
	err = os.WriteFile(req.ArtifactPath, []byte("## Simplifications\nNone.\nVerdict: PASS\n"), 0o644)
	return core.BridgeResponse{}, err
}

func (b *composingBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestAgentRunBridge_TheSeatPromptHoldsTheRequiredSectionsOfThePhaseContract(t *testing.T) {
	t.Parallel()
	root := diffReviewerProject(t, "code-simplifier")
	cb := &composingBridge{}
	run := agentcmd.New(agentcmd.Deps{
		InstallRouter: rootRouterInstaller,
		Bridge: func(r string, s *signalcenter.Center) core.Bridge {
			cb.real = agentRunBridge(r, s)
			return cb
		},
		Signals:   quietSignals,
		TreeState: fixedTreeState("tree"),
		Getenv:    func(string) string { return "" },
	})
	var stdout, stderr bytes.Buffer

	code := run([]string{"run", "code-simplifier", "--project-root", root}, strings.NewReader("diff"), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(cb.resolved, "<required-sections>\n    <section>## Simplifications</section>") {
		t.Errorf("the composed seat prompt lacks the required-sections block:\n%s", cb.resolved)
	}
}

func TestAgentRunBridge_AnUnreadableCatalogStillBuildsABridge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "phases", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "phases", "broken", "phase.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if agentRunBridge(root, nil) == nil {
		t.Error("agentRunBridge must return a bridge")
	}
}
