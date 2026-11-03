package main

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestLoopBatchRuntime_CloseRemovesTheSocketEnvItSet(t *testing.T) {
	t.Setenv(bridge.TmuxSocketEnv, "")
	rt := startLoopBatchRuntime()
	if os.Getenv(bridge.TmuxSocketEnv) == "" {
		t.Fatal("startLoopBatchRuntime did not export the per-run socket")
	}
	rt.close()
	if v, set := os.LookupEnv(bridge.TmuxSocketEnv); set {
		t.Errorf("close left %s=%q in the process environment", bridge.TmuxSocketEnv, v)
	}
}

func TestLoopBatchRuntime_CloseKeepsASocketTheCallerSet(t *testing.T) {
	t.Setenv(bridge.TmuxSocketEnv, "evolve-bridge-caller")
	rt := startLoopBatchRuntime()
	rt.close()
	if got := os.Getenv(bridge.TmuxSocketEnv); got != "evolve-bridge-caller" {
		t.Errorf("close changed a caller-owned socket to %q", got)
	}
}
