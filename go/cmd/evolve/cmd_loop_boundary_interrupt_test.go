package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestPrepareIteration_AnInterruptDuringTheRefreshStopsTheRunInsteadOfReExecing(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chainRebuildFn = func(string) error { cancel(); return nil }
	reExeced := false
	chainReExecFn = func(string, []string, []string) error { reExeced = true; return nil }
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: ctx, cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = &fixtures.FakeStorage{}
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)
	b.deps.Signals.Flush()

	if reExeced {
		t.Fatal("the interrupt arrived during the rebuild, so the refresh must not re-exec")
	}
	if d.flow != batchReturn || d.exitCode != 130 || b.result.StopReason != "signal" {
		t.Fatalf("the run ends with the signal stop: flow=%v exit=%d stop=%q\n%s", d.flow, d.exitCode, b.result.StopReason, console.String())
	}
	if out := console.String(); !strings.Contains(out, "received interrupt (SIGINT/SIGTERM) during the boundary refresh before cycle 2") || !strings.Contains(out, "step=interrupted") {
		t.Errorf("the console names the interrupted refresh and the stop: %s", out)
	}
}

func TestPrepareIteration_ALiveRefreshStillReExecs(t *testing.T) {
	root, evolveDir, _ := brhProject(t, "STALE_PIN", "REBUILT-BINARY-BYTES")
	u13StubRefresh(t, func() bool { return true })
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: io.Discard}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, io.Discard)
	b.deps.Storage = &fixtures.FakeStorage{}
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := b.prepareIteration(1, &fc, &bin, 0)

	if d.flow != batchReturn || d.exitCode != 0 || b.result.StopReason != "loop_boundary_refresh_reexec" {
		t.Fatalf("a live context re-execs as before: flow=%v exit=%d stop=%q", d.flow, d.exitCode, b.result.StopReason)
	}
}
