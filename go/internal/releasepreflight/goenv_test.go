package releasepreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func envDumpingGo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho GO_SHIM_RAN\nenv\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := defaultGoBinFn
	t.Cleanup(func() { defaultGoBinFn = old })
	defaultGoBinFn = func() string { return shim }
	return repo
}

func TestGoTestRunners_ScrubTheLaneIPCEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a shim subprocess")
	}
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "/plane/.evolve/runs/cycle-1700/cycle-state.json")
	t.Setenv("EVOLVE_BYPASS_SHIP_GATE", "1")
	repo := envDumpingGo(t)
	runners := map[string]func() error{
		"gate tests": func() error { return defaultGateTestRunner(repo, "./internal/guards/...") },
		"simulation": func() error { return defaultSimulationRunner(repo) },
	}
	for name, run := range runners {
		t.Run(name, func(t *testing.T) {
			err := run()
			if err == nil || !strings.Contains(err.Error(), "GO_SHIM_RAN") {
				t.Fatalf("err = %v, want the failing go shim's output: every go test the preflight spawns resolves the one go binary", err)
			}
			for _, key := range []string{ipcenv.FleetKey, ipcenv.CycleStateFileKey, "EVOLVE_BYPASS_SHIP_GATE"} {
				if strings.Contains(err.Error(), key+"=") {
					t.Errorf("%s reached the go test the preflight spawned", key)
				}
			}
		})
	}
}
