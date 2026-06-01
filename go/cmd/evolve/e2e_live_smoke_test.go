//go:build e2e

// Tier 0 — LIVE SMOKE. The cheapest real-CLI proof: a single `evolve bridge
// launch` call (one LLM turn) per available CLI, asserting the real binary
// boots, accepts the prompt, and writes a parseable artifact. ollama is FREE
// and the CI-safe canary. Gate: EVOLVE_E2E_LIVE_SMOKE=1.
package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// cheapModelFor returns the concrete cheapest model string for a CLI's bridge
// launch. Overridable per-CLI via EVOLVE_E2E_LIVE_MODEL_<BASECLI> so a host with
// different pulled/available models can tune without code changes.
func cheapModelFor(driver string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(driver, "-tmux"), "-p")
	if v := os.Getenv("EVOLVE_E2E_LIVE_MODEL_" + strings.ToUpper(base)); v != "" {
		return v
	}
	switch base {
	case "claude":
		return "haiku"
	case "codex":
		return "gpt-5.4-mini"
	case "agy":
		return "gemini-3.5-flash"
	case "ollama":
		return "llama3.1:8b"
	default:
		return "fast"
	}
}

func TestE2ELiveSmoke(t *testing.T) {
	liveGate(t, "EVOLVE_E2E_LIVE_SMOKE")
	repoRoot := mustRepoRoot(t)
	evolveBin := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	// ollama first (free canary), then the paid headless CLIs. tmux smoke is
	// redundant with the headless single-call here; the full tmux path is T1.
	targets := append([]liveCLI{
		{Driver: "ollama-tmux", Binary: "ollama", CheapTier: "fast", Family: "local"},
	}, liveHeadlessCLIs...)

	for _, cli := range targets {
		cli := cli
		t.Run(cli.Driver, func(t *testing.T) {
			if ok, why := liveCLIAvailable(cli); !ok {
				t.Skip(why)
			}
			smokeOneCLI(t, repoRoot, evolveBin, cli)
		})
	}
}

func smokeOneCLI(t *testing.T, repoRoot, evolveBin string, cli liveCLI) {
	t.Helper()
	timeout := envDurationSeconds("EVOLVE_E2E_LIVE_SMOKE_TIMEOUT_S", 3*time.Minute)
	size, out, err := liveBridgeLaunch(t, evolveBin, cli.Driver, cheapModelFor(cli.Driver), timeout)

	if err != nil {
		if isTransient(out, err) {
			t.Skipf("%s live smoke hit a transient/provider failure (quarantined):\nerr=%v\n%s", cli.Driver, err, lastN(out, 800))
		}
		t.Fatalf("%s live smoke failed (contract):\nerr=%v\n%s", cli.Driver, err, lastN(out, 1500))
	}
	if size < 0 {
		t.Fatalf("%s live smoke: artifact not written\n%s", cli.Driver, lastN(out, 1200))
	}
	if size == 0 {
		t.Errorf("%s live smoke: artifact is empty", cli.Driver)
	}
	t.Logf("[live-smoke] %s OK — artifact %d bytes", cli.Driver, size)
}
