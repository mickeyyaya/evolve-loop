package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_PromptSubmitWedged_DeliveryCauseSurvivesClassifier(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	const prompt = "Please produce the retrospective deliverable for this cycle now."
	eng := newTestEngine(Deps{
		Tmux: &parkedPromptTmux{
			fakeTmux:   &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
			parkedText: prompt,
		},
		Sleep:              func(time.Duration) {},
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 4,
		LookupEnv:          mapLookup(nil),
	})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: prompt,
		Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"),
		Agent: "retro", PermissionMode: "plan",
	})
	if cause := core.DeliveryFailureCause(err); !strings.Contains(cause, deliveryFailureReasonToken) {
		t.Fatalf("DeliveryFailureCause(Engine.Launch error) = %q, want %q classification; err=%v", cause, deliveryFailureReasonToken, err)
	}
}

func TestEngineLaunch_SilentPaneTimeout_NoDeliveryCause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	eng := newTestEngine(Deps{
		Tmux:               &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		Sleep:              func(time.Duration) {},
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 4,
		LookupEnv:          mapLookup(nil),
	})

	_, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: "x",
		Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"),
		Agent: "retro", PermissionMode: "plan",
	})
	if err == nil {
		t.Fatal("expected a generic artifact-timeout error")
	}
	if cause := core.DeliveryFailureCause(err); cause != "" {
		t.Fatalf("DeliveryFailureCause(generic silent timeout) = %q, want empty; err=%v", cause, err)
	}
}
