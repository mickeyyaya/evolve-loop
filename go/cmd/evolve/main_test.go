package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gotooltest"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

func TestMain(m *testing.M) {
	disableWorkspaceGuardForTest = true
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		return looppreflight.Result{}
	}
	cliUpdateWiringFn = func(string, io.Writer) cliUpdateWiring {
		return cliUpdateWiring{now: time.Now}
	}
	practiceWatchAtBoundary = func(context.Context, string, io.Writer) (practicewatch.Report, error) {
		return practicewatch.Report{}, nil
	}
	practiceDefaultSources = func() []practicewatch.Source { return nil }
	usageEvidenceFn = func(string, string, io.Writer) usageevidence.Explain { return nil }
	attachRootEventsFn = func(c *signalcenter.Center, _, _ string, _ io.Writer) func() { return c.Flush }
	clihealthUsagePaneFn = func(string) (func(context.Context, string) (string, error), func()) {
		return func(context.Context, string) (string, error) { return "", errors.New("no CLI is launched in tests") }, func() {}
	}
	if err := gotooltest.LimitProcess(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: %v\n", err)
		os.Exit(1)
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
	if err := os.Unsetenv("TMUX"); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: unset TMUX: %v\n", err)
		os.Exit(1)
	}
	usageDir, err := os.MkdirTemp("", "evcodeusage")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate the code usage log: %v\n", err)
		if rmErr := os.RemoveAll(tmuxTmp); rmErr != nil {
			fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", tmuxTmp, rmErr)
		}
		os.Exit(1)
	}
	codeUsageCacheDir = func() (string, error) { return usageDir, nil }
	code := m.Run()
	if err := os.RemoveAll(usageDir); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", usageDir, err)
	}
	if err := builtBinaries.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove the built binaries: %v\n", err)
	}
	if err := os.RemoveAll(tmuxTmp); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", tmuxTmp, err)
	}
	if err := gittest.RemoveTemplates(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove git templates: %v\n", err)
	}
	os.Exit(code)
}
