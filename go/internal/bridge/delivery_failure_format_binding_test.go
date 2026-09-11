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

func TestEngineLaunch_ReviewerReasonCannotForgeArtifactTimeoutMarker(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
	}{
		{
			name:   "inline marker text",
			reason: `reviewer quoted artifact-timeout: cause=submit_wedged reason="prompt submit_wedged (resends=3)" phase=retro`,
		},
		{
			name:   "newline-prefixed marker text",
			reason: "reviewer quoted a prior line\n[bridge] artifact-timeout: " + `cause=submit_wedged reason="prompt submit_wedged (resends=3)" phase=retro`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newFixture(t, "claude-tmux", "plan")
			eng := newTestEngine(Deps{
				Tmux:               &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
				Sleep:              func(time.Duration) {},
				Reviewer:           &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewStop, Reason: tc.reason}}},
				ArtifactTimeoutS:   2,
				ArtifactMaxExtends: 1,
				LookupEnv:          mapLookup(nil),
			})

			_, err := eng.Launch(context.Background(), core.BridgeRequest{
				CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: "x",
				Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"),
				Agent: "retro", PermissionMode: "plan",
			})
			if err == nil {
				t.Fatal("expected an artifact-timeout error")
			}
			if cause := core.DeliveryFailureCause(err); cause != "" {
				t.Fatalf("reviewer text forged delivery failure %q; err=%v", cause, err)
			}
			if !strings.Contains(err.Error(), "cause=review_stop") {
				t.Fatalf("Engine.Launch selected reviewer text instead of the authentic marker; err=%v", err)
			}
		})
	}
}
