package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bypass"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

func TestRunShipCmd_AutoConfirmIsAnAuditedBypass(t *testing.T) {
	calls := stubShipRun(t, ship.RunResult{}, nil)
	root := t.TempDir()
	var stderr bytes.Buffer

	rc := runShipCmd([]string{"--project-root", root, "--class", "manual", "--auto-confirm=hotfix of a red main", "msg"}, nil, io.Discard, &stderr)

	if rc != 0 || len(*calls) != 1 || (*calls)[0].AutoConfirm != "hotfix of a red main" {
		t.Fatalf("rc=%d calls=%+v stderr=%q, want one ship run with the reason", rc, *calls, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil || !strings.Contains(string(raw), `"kind":"`+bypass.LedgerKind+`"`) || !strings.Contains(string(raw), "hotfix of a red main") {
		t.Fatalf("ledger = %q (%v), want one control_bypass record with the reason", raw, err)
	}
	if !strings.Contains(stderr.String(), "[bypass] --auto-confirm used by") {
		t.Errorf("stderr = %q, want the bypass line", stderr.String())
	}
}

func TestRunShipCmd_AutoConfirmIsRefusedInsideACell(t *testing.T) {
	calls := stubShipRun(t, ship.RunResult{}, nil)
	t.Setenv(ipcenv.PhaseKey, "build")
	var stderr bytes.Buffer

	rc := runShipCmd([]string{"--project-root", t.TempDir(), "--class", "manual", "--auto-confirm=x", "msg"}, nil, io.Discard, &stderr)

	if rc != 1 || len(*calls) != 0 || !strings.Contains(stderr.String(), "inside an LLM cell") {
		t.Fatalf("rc=%d calls=%d stderr=%q, want a refusal before ship runs", rc, len(*calls), stderr.String())
	}
}

func TestRunShipCmd_AutoConfirmOutsideAManualRunWritesNoRecord(t *testing.T) {
	calls := stubShipRun(t, ship.RunResult{}, nil)
	root := t.TempDir()
	var stderr bytes.Buffer

	if rc := runShipCmd([]string{"--project-root", root, "--class", "release", "--auto-confirm=x", "msg"}, nil, io.Discard, &stderr); rc != 1 || !strings.Contains(stderr.String(), "only to --class manual") {
		t.Errorf("release class: rc=%d stderr=%q, want a refusal", rc, stderr.String())
	}
	if rc := runShipCmd([]string{"--project-root", root, "--class", "manual", "--dry-run", "--auto-confirm=x", "msg"}, nil, io.Discard, io.Discard); rc != 0 || len(*calls) != 1 {
		t.Errorf("dry run: rc=%d calls=%d, want one dry run", rc, len(*calls))
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve", "ledger.jsonl")); !os.IsNotExist(err) {
		t.Errorf("ledger stat err = %v, want no ledger: neither run used the bypass", err)
	}
}
