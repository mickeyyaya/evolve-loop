package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

// TestMain disables the workspace-pollution guard, since this package's
// tests pre-seed workspace files the guard would otherwise archive away, and
// points TMUX_TMPDIR at a private dir so the loop's orphan-socket sweep never
// reaps the host's tmux sockets (a short /tmp base keeps socket paths under
// the AF_UNIX length limit).
func TestMain(m *testing.M) {
	disableWorkspaceGuardForTest = true
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		return looppreflight.Result{}
	}
	cliUpdateWiringFn = func(string, io.Writer) cliUpdateWiring {
		return cliUpdateWiring{now: time.Now}
	}
	usageEvidenceFn = func(string, string, io.Writer) usageevidence.Explain { return nil }
	clihealthUsagePaneFn = func(string) (func(context.Context, string) (string, error), func()) {
		return func(context.Context, string) (string, error) { return "", errors.New("no CLI is launched in tests") }, func() {}
	}
	tmuxTmp, err := os.MkdirTemp("/tmp", "evtmux")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate TMUX_TMPDIR: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("TMUX_TMPDIR", tmuxTmp); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: set TMUX_TMPDIR: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(tmuxTmp); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", tmuxTmp, err)
	}
	os.Exit(code)
}
