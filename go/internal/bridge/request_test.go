package bridge

// request_test.go — ADR-0103 unit 10, review fold (architecture MEDIUM 2):
// the required-field gauntlet lives in the HOST — a pre-launch rule beside
// the Launch that runs it, not in the outcome classifier — and the Adapter
// projects it through gobridge.ValidateRequest. Test 32 moved verbatim from
// launchoutcome/request_test.go.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Test 32 — CLI, Profile, Workspace, ArtifactPath in that order, each its
// exact string; nil when all are set.
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
