package bridge

import (
	"errors"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func errorsIsArtifactTimeout(err error) bool { return errors.Is(err, core.ErrArtifactTimeout) }
