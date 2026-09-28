package loopchain

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func namedCurrentWorkspace(t *testing.T, evolveDir, name string) string {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := json.Marshal(map[string]any{"cycle_id": 1706, "workspace_path": dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), state, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLiveSiblingRun_NamesTheBlockingRunAndWhy(t *testing.T) {
	sibling := exec.Command("sleep", "30")
	if err := sibling.Start(); err != nil {
		t.Fatalf("spawn a live sibling: %v", err)
	}
	t.Cleanup(func() { _ = sibling.Process.Kill(); _ = sibling.Wait() })
	for _, tc := range []struct {
		name   string
		seed   func(t *testing.T, evolveDir string) string
		reason string
	}{
		{"a live run another process owns", func(t *testing.T, evolveDir string) string {
			liveRun(t, evolveDir, "cycle-1706", sibling.Process.Pid)
			return filepath.Join(evolveDir, "runs", "cycle-1706")
		}, "live pid " + strconv.Itoa(sibling.Process.Pid)},
		{"a fresh lease with no owner pid", func(t *testing.T, evolveDir string) string {
			liveRun(t, evolveDir, "cycle-1706", 0)
			return filepath.Join(evolveDir, "runs", "cycle-1706")
		}, "a fresh lease with no owner pid"},
		{"the named run has no lease", func(t *testing.T, evolveDir string) string {
			return namedCurrentWorkspace(t, evolveDir, "cycle-1706")
		}, "no readable lease"},
		{"the named run's lease is torn", func(t *testing.T, evolveDir string) string {
			dir := namedCurrentWorkspace(t, evolveDir, "cycle-1706")
			if err := os.WriteFile(filepath.Join(dir, ".lease"), []byte("{"), 0o644); err != nil {
				t.Fatal(err)
			}
			return dir
		}, "no readable lease (runlease: parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			dir := tc.seed(t, evolveDir)

			run, active, err := LiveSiblingRun(evolveDir)

			if err != nil || !active || run.Dir != dir || !strings.HasPrefix(run.Reason, tc.reason) {
				t.Fatalf("LiveSiblingRun = (%+v, %v, %v), want %s blocking with reason %q", run, active, err, dir, tc.reason)
			}
			if fleet, ferr := FleetLaneActive(evolveDir); ferr != nil || !fleet {
				t.Fatalf("FleetLaneActive = (%v, %v), want it to agree", fleet, ferr)
			}
		})
	}
}

func TestLiveSiblingRun_TheCallersOwnAndASealedLaneDoNotBlock(t *testing.T) {
	exited := exec.Command("true")
	if err := exited.Run(); err != nil {
		t.Skipf("cannot spawn a process to retire: %v", err)
	}
	evolveDir := t.TempDir()
	liveRun(t, evolveDir, "cycle-own", os.Getpid())
	liveRun(t, evolveDir, "cycle-sealed", exited.Process.Pid)

	run, active, err := LiveSiblingRun(evolveDir)

	if err != nil || active || run != (SiblingRun{}) {
		t.Fatalf("LiveSiblingRun = (%+v, %v, %v), want no sibling", run, active, err)
	}
}

func TestLiveSiblingRun_ADiscoveryErrorIsReturned(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, active, err := LiveSiblingRun(evolveDir)

	if err == nil || active || !strings.Contains(err.Error(), "fleet-lane discovery") {
		t.Fatalf("LiveSiblingRun = (%v, %v), want the discovery error", active, err)
	}
}
