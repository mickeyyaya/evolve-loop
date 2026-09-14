package ship

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest reproduces lane
// 1677's false RED (2026-09-14): a fleet lane exports EVOLVE_FLEET=1 and
// EVOLVE_CYCLE_STATE_FILE=<its own run dir> process-wide, and the repo-contract
// gate's `go test` inherited both — so cmd/evolve's cycle-reset lease tests,
// guards' "outside a cycle" tests and ship's fleet-off goldens failed in the
// lane worktree while passing in CI and in a clean shell. The gate must hand
// `go test` the environment CI has: the lane's IPC namespace scrubbed.
func TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "/runs/cycle-1677/cycle-state.json")

	module := envProbeModule(t, ipcenv.FleetKey, ipcenv.CycleStateFileKey)
	var out bytes.Buffer
	outcome := runRepoContractPackages(context.Background(), module, &out, []string{"./..."})

	if outcome.err != nil || len(outcome.failedTests) != 0 {
		t.Fatalf("the gate leaked the lane's IPC env into go test: err=%v failed=%v\n%s", outcome.err, outcome.failedTests, out.String())
	}
	if _, err := os.Stat(filepath.Join(module, probeRanMarker)); err != nil {
		t.Fatalf("probe test did not run (%v):\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "TestNoLaneIPCEnvLeaks") {
		t.Fatalf("the scan tee lost the probe's output (stderr copy and tee race on one writer):\n%s", out.String())
	}
}

// probeRanMarker is written by the probe test itself, so the proof it ran does
// not depend on the scan tee; the tee assertion above then pins the runner's
// lockedWriter (without it, under -race this test reports a data race between
// exec's stderr copy and the tee on the one bytes.Buffer, and the tee text is
// lost at EOF).
const probeRanMarker = "probe-ran.marker"

// envProbeModule writes a throwaway Go module whose single test fails when any
// of keys is present in its environment — the shape of every env-sensitive test
// the gate runs, reduced to the leak itself.
func envProbeModule(t *testing.T, keys ...string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module envprobe\n\ngo 1.23\n")
	mustWrite(t, filepath.Join(dir, "envprobe_test.go"), fmt.Sprintf(`package envprobe

import (
	"os"
	"testing"
)

func TestNoLaneIPCEnvLeaks(t *testing.T) {
	if err := os.WriteFile(%q, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{%s} {
		if v, ok := os.LookupEnv(k); ok {
			t.Fatalf("%%s=%%q leaked into the gate's go test", k, v)
		}
	}
}
`, probeRanMarker, quoteAll(keys)))
	return dir
}

func quoteAll(keys []string) string {
	q := make([]string, len(keys))
	for i, k := range keys {
		q[i] = fmt.Sprintf("%q", k)
	}
	return strings.Join(q, ", ")
}
