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

func TestRunRepoContractPackages_ScrubsTheLaneIPCEnvFromGoTest(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.CycleStateFileKey, "/runs/cycle-1677/cycle-state.json")

	module := envProbeModule(t, ipcenv.FleetKey, ipcenv.CycleStateFileKey)
	var out bytes.Buffer
	outcome := runRepoContractPackages(context.Background(), module, packLog{notes: &out, raw: &out}, []string{"./..."})

	if outcome.err != nil || len(outcome.failures) != 0 {
		t.Fatalf("the gate leaked the lane's IPC env into go test: err=%v failed=%v\n%s", outcome.err, outcome.failedNames(), out.String())
	}
	if _, err := os.Stat(filepath.Join(module, probeRanMarker)); err != nil {
		t.Fatalf("probe test did not run (%v):\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "TestNoLaneIPCEnvLeaks") {
		t.Fatalf("the scan tee lost the probe's output (stderr copy and tee race on one writer):\n%s", out.String())
	}
}

const probeRanMarker = "probe-ran.marker"

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
