package cyclesimulator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeScript writes an executable bash script at <pluginRoot>/legacy/scripts/<rel>.
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

// TestRun_DefaultShipFn_ExitZero exercises the default (non-injected) ShipDryRunFn
// closure against a real ship.sh that exits 0 — covering the closure body and its
// ProcessState.ExitCode()==0 return path. AdvanceFn/VerifyFn are still injected so
// the only shell-out under test is the ship default.
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
		// ShipDryRunFn left nil → default closure runs the real ship.sh
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "ship.sh --dry-run completed cleanly") {
		t.Errorf("expected clean-ship log, got: %s", stderr.String())
	}
}

// TestRun_DefaultShipFn_NonZero exercises the default ShipDryRunFn against a
// ship.sh that exits 2 — covering the ProcessState.ExitCode()!=0 branch and the
// "acceptable for tree-state-mismatch" tolerated path (run still returns ExitOK).
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

// TestRun_DefaultVerifyFn_Success exercises the default (non-injected) VerifyFn
// closure against a real verify-ledger-chain.sh that exits 0 — covering the
// closure body and the "OK: ledger chain intact" success log.
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
		// VerifyFn left nil → default closure runs the real verify script
	}, &stderr)

	if rc != ExitOK {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitOK, stderr.String())
	}
	if !strings.Contains(stderr.String(), "OK: ledger chain intact") {
		t.Errorf("expected chain-intact log, got: %s", stderr.String())
	}
}

// TestAppendSimLedger_OpenFileCreateDenied covers the OpenFile error branch
// (327-329) reached AFTER a clean readChainLink: the ledger file does not yet
// exist (so the chain-link read returns the zero seed without error), the ledger
// directory exists but is read-only, so O_CREATE|O_WRONLY is denied.
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
	// Make the ledger dir read-only AFTER creation so MkdirAll(dir) is a no-op
	// success but creating a new file inside is denied.
	if err := os.Chmod(evolveDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(evolveDir, "ledger.jsonl") // does not exist yet
	if err := appendSimLedger(ledgerPath, 1, "scout", art, "tok", root, fixedNow); err == nil {
		t.Error("expected OpenFile create-denied error in a read-only ledger dir")
	}
}

// TestJSONCompact_ValueMarshalError covers the value-marshal error branch in
// jsonCompact: a value that encoding/json cannot marshal (a channel) under a
// canonical key forces json.Marshal(v) to fail and jsonCompact to return the error.
func TestJSONCompact_ValueMarshalError(t *testing.T) {
	t.Parallel()
	m := map[string]any{
		"ts":    "2026-01-01T00:00:00Z",
		"cycle": make(chan int), // channels are not JSON-marshalable
	}
	if _, err := jsonCompact(m); err == nil {
		t.Error("expected jsonCompact to error on an unmarshalable value")
	}
}
