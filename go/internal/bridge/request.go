package bridge

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// ValidateRequest is the required-field gauntlet Launch runs first and the
// production Adapter runs before every Launch: CLI, Profile, Workspace,
// ArtifactPath in that order, each named by its exact "bridge: <Field>
// required" string; nil when all four are set.
func ValidateRequest(req core.BridgeRequest) error {
	switch "" {
	case req.CLI:
		return errors.New("bridge: CLI required")
	case req.Profile:
		return errors.New("bridge: Profile required")
	case req.Workspace:
		return errors.New("bridge: Workspace required")
	case req.ArtifactPath:
		return errors.New("bridge: ArtifactPath required")
	}
	return nil
}
