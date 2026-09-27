package bridge

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestValidateRequest_FourRequiredFieldsInOrder(t *testing.T) {
	for _, tc := range []struct {
		req  core.BridgeRequest
		want string
	}{
		{core.BridgeRequest{}, "bridge: CLI required"},
		{core.BridgeRequest{CLI: "claude-p"}, "bridge: Profile required"},
		{core.BridgeRequest{CLI: "claude-p", Profile: "p"}, "bridge: Workspace required"},
		{core.BridgeRequest{CLI: "claude-p", Profile: "p", Workspace: "w"}, "bridge: ArtifactPath required"},
		{core.BridgeRequest{Profile: "p", Workspace: "w", ArtifactPath: "a"}, "bridge: CLI required"},
	} {
		if err := ValidateRequest(tc.req); err == nil || err.Error() != tc.want {
			t.Errorf("ValidateRequest(%+v) = %v, want %q", tc.req, err, tc.want)
		}
	}
	if err := ValidateRequest(core.BridgeRequest{CLI: "claude-p", Profile: "p", Workspace: "w", ArtifactPath: "a"}); err != nil {
		t.Errorf("all set: %v", err)
	}
}
