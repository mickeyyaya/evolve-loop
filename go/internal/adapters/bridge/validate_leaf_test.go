package bridge

import (
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestValidate_ProjectsTheHostRule(t *testing.T) {
	for _, req := range []core.BridgeRequest{
		{},
		{CLI: "claude-p"},
		{CLI: "claude-p", Profile: "p"},
		{CLI: "claude-p", Profile: "p", Workspace: "w"},
		{CLI: "claude-p", Profile: "p", Workspace: "w", ArtifactPath: "a"},
	} {
		got, want := validate(req), gobridge.ValidateRequest(req)
		switch {
		case got == nil && want == nil:
		case got == nil || want == nil || got.Error() != want.Error():
			t.Errorf("validate(%+v) = %v, the host says %v", req, got, want)
		}
	}
	if err := validate(core.BridgeRequest{CLI: "claude-p"}); err == nil || err.Error() != "bridge: Profile required" {
		t.Errorf("the strings are the engine's: %v", err)
	}
}
