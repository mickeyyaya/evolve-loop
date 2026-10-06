package looppreflight

import (
	"os"
	"path/filepath"
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
