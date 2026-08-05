package main

// cmd_loop_fingerprint_ack_test.go — caller proof for the cycle-1332
// blocker-breaker fingerprint-ack CLI flag: `evolve loop --reset
// --fingerprint <fp>`, driven from the REAL production entrypoint (runLoop),
// must append the ack ledger record. A predicate that calls
// core.AppendResolvedFingerprint directly proves nothing about the
// operator-facing flag actually being wired end-to-end.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRunLoop_FingerprintAck_AppendsLedgerRecord(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--reset",
		"--fingerprint", "ship|unknown|76d0f4fca190",
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}

	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("resolved-fingerprints.json not written: %v", err)
	}
	if !strings.Contains(string(raw), "ship|unknown|76d0f4fca190") {
		t.Fatalf("ledger must contain the acked fingerprint, got %s", raw)
	}
	if !strings.Contains(stderr.String(), "acknowledged") {
		t.Errorf("operator-facing log must confirm the ack, got %q", stderr.String())
	}
}
