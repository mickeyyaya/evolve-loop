package shiptest_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func runShip(t *testing.T, repo string, opts ship.Options) (ship.RunResult, error) {
	t.Helper()
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = repo
	}
	if opts.PluginRoot == "" {
		opts.PluginRoot = repo
	}
	if opts.ShipBinaryPath == "" {
		opts.ShipBinaryPath = filepath.Join(repo, "ship-binary-fixture")
	}
	var stderr bytes.Buffer
	if opts.Stderr == nil {
		opts.Stderr = &stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = bytes.NewReader(nil)
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ship.Run(ctx, opts)
}

func containsLog(res ship.RunResult, substr string) bool {
	return shiptest.ContainsLog(res.Logs, substr)
}
