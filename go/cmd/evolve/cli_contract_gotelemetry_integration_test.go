//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLIContract_GCSuccessLeavesNoGoTelemetryUnderTheTestHome(t *testing.T) {
	contractEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if out, err := exec.Command("go", "env", "GOROOT").CombinedOutput(); err != nil {
		t.Fatalf("go env GOROOT through the contract PATH: %v\n%s", err, out)
	}
	telemetry := filepath.Join(home, "Library", "Application Support", "go", "telemetry")
	if _, err := os.Stat(telemetry); err == nil {
		t.Errorf("go telemetry was written under the test HOME: %s", telemetry)
	}
}
