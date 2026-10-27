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
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type sandboxSwitchSpy struct{ off bool }

func (s *sandboxSwitchSpy) SetSandboxOff() { s.off = true }

func TestApplySandboxOff_RecordsTheReasonAndTurnsTheSandboxOff(t *testing.T) {
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	evolveDir := t.TempDir()
	center := signalcenter.New()
	spy := &sandboxSwitchSpy{}
	var stderr bytes.Buffer

	if err := newBypassAuditor(evolveDir, center, &stderr).applySandboxOff(spy, "debug the pane on this host"); err != nil || !spy.off {
		t.Fatalf("err=%v off=%v, want the sandbox off", err, spy.off)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "ledger.jsonl"))
	if err != nil || !strings.Contains(string(raw), `"action":"sandbox-off"`) || !strings.Contains(string(raw), "debug the pane on this host") {
		t.Fatalf("ledger = %q (%v), want one sandbox-off record with the reason", raw, err)
	}
	center.Flush()
	if got := center.Recent(); len(got) == 0 || got[len(got)-1].Code != bypass.CodeUsed {
		t.Errorf("signals = %+v, want %s", got, bypass.CodeUsed)
	}
}

func TestApplySandboxOff_NoReasonKeepsTheSandboxAndACellOrABrokenLedgerRefuses(t *testing.T) {
	spy := &sandboxSwitchSpy{}
	if err := newBypassAuditor(t.TempDir(), nil, io.Discard).applySandboxOff(spy, ""); err != nil || spy.off {
		t.Fatalf("no reason: err=%v off=%v, want no change", err, spy.off)
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := newBypassAuditor(blocker, nil, io.Discard).applySandboxOff(spy, "x"); err == nil || !strings.Contains(err.Error(), "audit record failed") || spy.off {
		t.Fatalf("broken ledger: err=%v off=%v, want a refusal before the sandbox goes off", err, spy.off)
	}
	t.Setenv(ipcenv.PhaseKey, "build")
	if err := newBypassAuditor(t.TempDir(), nil, io.Discard).applySandboxOff(spy, "x"); err == nil || spy.off {
		t.Fatalf("in a cell: err=%v off=%v, want a refusal", err, spy.off)
	}
}

func TestSandboxOffFlags_ReachTheRunAndThePreflight(t *testing.T) {
	t.Parallel()
	cfg, rc := parseLoopArgs([]string{"--goal-text", "x", "--cycles", "1", "--sandbox-off=nested host"}, io.Discard)
	if rc != 0 || cfg.SandboxOff != "nested host" || cfg.sandboxMode() != "off" {
		t.Fatalf("loop: rc=%d SandboxOff=%q mode=%q", rc, cfg.SandboxOff, cfg.sandboxMode())
	}
	if (loopConfig{}).sandboxMode() != "" {
		t.Error("no flag must leave the preflight mode empty")
	}
	f, ok := parseCycleRunFlags([]string{"--goal-hash", "abcd1234", "--sandbox-off=nested host"}, io.Discard)
	if !ok || f.sandboxOff != "nested host" {
		t.Fatalf("cycle run: ok=%v sandboxOff=%q", ok, f.sandboxOff)
	}
}

func TestWireOrchestratorDeps_ARefusedSandboxBypassStopsTheWiring(t *testing.T) {
	t.Setenv(ipcenv.PhaseKey, "build")
	root, evolveDir := wiringRoot(t, "")

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{sandboxOff: "x"})
	defer d.CloseSignals()

	if d.RoutingErr == nil || !strings.Contains(d.RoutingErr.Error(), "inside an LLM cell") || d.Orchestrator != nil {
		t.Fatalf("wiring = (err=%v orch=%v), want the in-cell refusal before any dispatch", d.RoutingErr, d.Orchestrator)
	}
}
