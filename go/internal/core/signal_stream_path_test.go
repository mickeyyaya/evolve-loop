package core

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestSignalStreamPath_ACycleNamesTheRunStreamAndNoCycleTheRootStream(t *testing.T) {
	t.Parallel()
	if got := SignalStreamPath("/p", 7); got != filepath.Join(paths.RunWorkspace("/p", 7), signalcenter.StreamFileName) {
		t.Fatalf("cycle stream %q", got)
	}
	if got := SignalStreamPath("/p", 0); got != filepath.Join(paths.EvolveDirOf("/p"), signalcenter.StreamFileName) {
		t.Fatalf("root stream %q", got)
	}
}
