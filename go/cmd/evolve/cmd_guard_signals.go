package main

import (
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func guardDenialCenter(evolveDir string) (*signalcenter.Center, func()) {
	return newRoleSignalCenter(roleGuard, filepath.Dir(evolveDir), evolveDir, io.Discard)
}
