package looppreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func widenVersionProbe(t *testing.T) {
	t.Helper()
	old := versionCaptureTimeout
	versionCaptureTimeout = time.Minute
	t.Cleanup(func() { versionCaptureTimeout = old })
}

func installFakeAgy(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agy"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AGY_CLI_DISABLE_AUTO_UPDATE", "")
}

func TestCLIVersion_TheRealProbeRunsAgyWithItsSelfUpdaterOff(t *testing.T) {
	widenVersionProbe(t)
	installFakeAgy(t, `[ "$AGY_CLI_DISABLE_AUTO_UPDATE" = 1 ] || { echo "would self-update first" >&2; exit 3; }
echo 1.2.17
`)

	v, err := CLIVersion("agy")

	if err != nil || v != "1.2.17" {
		t.Fatalf("`agy --version` must run with the agy-tmux manifest's default_env, which turns its self-updater off; got %q, %v", v, err)
	}
}

func TestCLIVersion_AtEnforceTheInventoryProbeGetsTheCellEnvWithoutCLAUDECODE(t *testing.T) {
	widenVersionProbe(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"isolation":{"i2":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("CLAUDECODE", "1")
	installFakeAgy(t, `[ -z "$CLAUDECODE" ] || { echo "nested claude" >&2; exit 5; }
echo 1.2.18
`)

	v, err := CLIVersion("agy")

	if err != nil || v != "1.2.18" {
		t.Fatalf("the inventory probe goes through the bridge launch API, so CLAUDECODE must not reach it; got %q, %v", v, err)
	}
}

func TestCLIVersion_ANonZeroExitIsAnError(t *testing.T) {
	widenVersionProbe(t)
	installFakeAgy(t, "echo 9.9.9\nexit 2\n")

	if _, err := CLIVersion("agy"); err == nil || !strings.Contains(err.Error(), "exit 2") {
		t.Fatalf("err = %v; want the exit code of the failed probe", err)
	}
}
