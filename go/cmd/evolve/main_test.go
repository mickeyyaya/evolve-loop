package main

import (
	"io"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

// TestMain disables the workspace-pollution guard, since this package's
// tests pre-seed workspace files the guard would otherwise archive away.
func TestMain(m *testing.M) {
	disableWorkspaceGuardForTest = true
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		return looppreflight.Result{}
	}
	os.Exit(m.Run())
}
