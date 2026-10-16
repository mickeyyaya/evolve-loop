package cyclesimulator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, pluginRoot, rel, body string) {
	t.Helper()
	full := filepath.Join(pluginRoot, "legacy", "scripts", rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestRun_DefaultShipFn_ExitZero(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	root := t.TempDir()
	pluginRoot := t.TempDir()
	writeScript(t, pluginRoot, "lifecycle/ship.sh", "#!/bin/bash\nexit 0\n")

	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:       1,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		PluginRoot:  pluginRoot,
		AdvanceFn:   func(string, string) error { return nil },
		VerifyFn:    func() error { return nil },
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "ship.sh --dry-run completed cleanly") {
		t.Errorf("expected clean-ship log, got: %s", stderr.String())
	}
}

func TestRun_DefaultShipFn_NonZero(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	root := t.TempDir()
	pluginRoot := t.TempDir()
	writeScript(t, pluginRoot, "lifecycle/ship.sh", "#!/bin/bash\nexit 2\n")

	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:       1,
		Workspace:   filepath.Join(root, "ws"),
		ProjectRoot: root,
		PluginRoot:  pluginRoot,
		AdvanceFn:   func(string, string) error { return nil },
		VerifyFn:    func() error { return nil },
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d (non-zero ship is tolerated) (log=%s)", rc, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exited rc=2") {
		t.Errorf("expected rc=2 tolerated log, got: %s", stderr.String())
	}
}

func TestRun_DefaultVerifyFn_Success(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	root := t.TempDir()
	pluginRoot := t.TempDir()
	writeScript(t, pluginRoot, "observability/verify-ledger-chain.sh", "#!/bin/bash\nexit 0\n")

	var stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:        1,
		Workspace:    filepath.Join(root, "ws"),
		ProjectRoot:  root,
		PluginRoot:   pluginRoot,
		AdvanceFn:    func(string, string) error { return nil },
		ShipDryRunFn: func(string) (int, error) { return 0, nil },
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "OK: ledger chain intact") {
		t.Errorf("expected chain-intact log, got: %s", stderr.String())
	}
}

func TestAppendSimLedger_OpenFileCreateDenied(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(evolveDir, "ledger.jsonl")
	if err := appendSimLedger(ledgerPath, 1, "scout", art, "tok", root, fixedNow); err == nil {
		t.Error("expected OpenFile create-denied error in a read-only ledger dir")
	}
}

func TestJSONCompact_ValueMarshalError(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"ts":    "2026-01-01T00:00:00Z",
		"cycle": make(chan int),
	}
	if _, err := jsonCompact(m); err == nil {
		t.Error("expected jsonCompact to error on an unmarshalable value")
	}
}
