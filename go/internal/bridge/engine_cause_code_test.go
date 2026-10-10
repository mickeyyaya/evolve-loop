package bridge

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestEngineLaunch_TheResponseCarriesTheClassifiedCauseCode(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	const prompt = "Please produce the retrospective deliverable for this cycle now."
	eng := newTestEngine(Deps{
		Tmux:               &parkedPromptTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}, parkedText: prompt},
		Sleep:              func(time.Duration) {},
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 4,
		LookupEnv:          mapLookup(nil),
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: fx.profile, Model: "auto", Prompt: prompt,
		Workspace: fx.ws, ArtifactPath: filepath.Join(fx.ws, "a.md"), Agent: "retro", PermissionMode: "plan",
	})

	if err == nil || resp.ExitCode != ExitArtifactTimeout || resp.CauseCode != "submit_wedged" {
		t.Fatalf("resp = exit %d cause %q (err %v), want exit 81 with the classified cause submit_wedged", resp.ExitCode, resp.CauseCode, err)
	}
}
