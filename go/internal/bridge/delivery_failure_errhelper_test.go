package bridge

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// errorsIsArtifactTimeout keeps the sentinel check in one place for the
// cycle-1562 delivery-failure contract: a delivery failure must remain an
// ErrArtifactTimeout (exit 81), because that is the sentinel
// core.IsInfraTeardownError matches to perform its ONE bounded relaunch.
// Reclassifying it to any other sentinel would silently remove the recovery
// this task exists to reach.
func errorsIsArtifactTimeout(err error) bool { return errors.Is(err, core.ErrArtifactTimeout) }
