//go:build !windows

package swarmrunner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestSwarmRunner_CoverageHelpers(t *testing.T) {
	t.Run("branchByID", func(t *testing.T) {
		got := branchByID(swarm.SwarmPlan{Workers: []swarm.WorkerSpec{
			{WorkerID: "w0", Branch: "cycle-282-w0"},
			{WorkerID: "w1", Branch: "cycle-282-w1"},
		}})
		if got["w0"] != "cycle-282-w0" || got["w1"] != "cycle-282-w1" {
			t.Fatalf("branchByID = %#v", got)
		}
	})

	t.Run("annotate nil signals and error", func(t *testing.T) {
		var resp core.PhaseResponse
		annotate(&resp, swarm.SwarmPlan{Mode: swarm.ModeWriter},
			swarm.SwarmResult{Workers: []swarm.WorkerResult{{WorkerID: "w0", ExitCode: 1}}},
			errors.New("dispatch failed"), "enforce")
		if resp.Signals["swarm.stage"] != "enforce" || resp.Signals["swarm.error"] != "dispatch failed" {
			t.Fatalf("annotate signals = %#v", resp.Signals)
		}
		if resp.Signals["swarm.all_ok"] != false {
			t.Fatalf("annotate all_ok = %#v", resp.Signals["swarm.all_ok"])
		}
	})

	t.Run("kill helpers defensive paths", func(t *testing.T) {
		if err := groupKiller(1); err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Fatalf("groupKiller must refuse pgid<=1, got %v", err)
		}
		if err := tmuxKiller(context.Background(), "definitely-missing-session"); err != nil {
			t.Fatalf("tmuxKiller is best-effort, got %v", err)
		}
	})

	t.Run("groupKiller kills a child process group", func(t *testing.T) {
		cmd := exec.Command("sleep", "30")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			t.Skipf("cannot start sleep for process-group kill coverage: %v", err)
		}
		if err := groupKiller(cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			t.Fatalf("groupKiller real process group: %v", err)
		}
		_ = cmd.Wait()
	})
}

func TestSwarmRunner_CoveragePlanLoadingAndDeps(t *testing.T) {
	t.Run("markdown plan fallback", func(t *testing.T) {
		ws := t.TempDir()
		body := "```json\n" +
			`{"swarm_plan":{"task_id":"t","mode":"reader","partitionable":true,"workers":[{"worker_id":"w0"},{"worker_id":"w1"}]}}` +
			"\n```\n"
		if err := os.WriteFile(filepath.Join(ws, "swarm-plan.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		plan, ok := loadPlan(ws)
		if !ok || plan.TaskID != "t" || len(plan.Workers) != 2 {
			t.Fatalf("loadPlan markdown = (%+v, %v)", plan, ok)
		}
	})

	t.Run("invalid plan collapses to not ok", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.WriteFile(filepath.Join(ws, "swarm-plan.json"), []byte("{bad json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := loadPlan(ws); ok {
			t.Fatal("invalid plan should not load")
		}
	})

	t.Run("writer deps include provisioner and port base", func(t *testing.T) {
		d := New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter)
		deps := d.dispatchDeps(core.PhaseRequest{
			Cycle: 282, Workspace: t.TempDir(),
			Env: map[string]string{"EVOLVE_SWARM_PORT_BASE": "62000"},
		})
		if deps.Provisioner == nil {
			t.Fatal("writer dispatch deps must include a provisioner")
		}
		if deps.PortBase != 62000 {
			t.Fatalf("PortBase = %d, want 62000", deps.PortBase)
		}
		if deps.Registry == nil || deps.Killer == nil || deps.Launcher == nil {
			t.Fatalf("incomplete deps: %+v", deps)
		}
	})

	t.Run("valid but non-partitionable plan falls back to inner", func(t *testing.T) {
		inner := &fakeInner{name: "build"}
		d := New(inner, &fakeBridge{}, swarm.ModeWriter)
		ws := t.TempDir()
		plan := `{"swarm_plan":{"task_id":"t","mode":"writer","partitionable":false,"workers":[{"worker_id":"w0"},{"worker_id":"w1"}]}}`
		if err := os.WriteFile(filepath.Join(ws, "swarm-plan.json"), []byte(plan), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Run(context.Background(), reqWith(ws, map[string]string{"EVOLVE_SWARM_STAGE": "enforce"})); err != nil {
			t.Fatal(err)
		}
		if got := atomic.LoadInt32(&inner.ran); got != 1 {
			t.Fatalf("non-partitionable plan should delegate to inner, ran=%d", got)
		}
	})
}

func TestSwarmRunner_CoverageEnforceFailureBranches(t *testing.T) {
	t.Run("dispatch error fails response", func(t *testing.T) {
		d := New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter)
		resp, err := d.enforce(context.Background(), reqWith(t.TempDir(), nil),
			swarm.SwarmPlan{Mode: swarm.ModeWriter},
			swarm.SwarmResult{},
			errors.New("launch failed"))
		if err == nil || resp.Verdict != core.VerdictFAIL {
			t.Fatalf("enforce dispatch error = verdict %s err %v", resp.Verdict, err)
		}
	})

	t.Run("writer merge failure fails response and records merge signal", func(t *testing.T) {
		d := New(&fakeInner{name: "build"}, &fakeBridge{}, swarm.ModeWriter)
		resp, err := d.enforce(context.Background(), reqWith(t.TempDir(), nil),
			swarm.SwarmPlan{
				Mode:              swarm.ModeWriter,
				IntegrationBranch: "cycle-282-integration",
				Workers: []swarm.WorkerSpec{
					{WorkerID: "w0", Branch: "cycle-282-w0"},
				},
			},
			swarm.SwarmResult{
				IntegrationWorktree: filepath.Join(t.TempDir(), "missing"),
				Workers:             []swarm.WorkerResult{{WorkerID: "w0", ExitCode: 0}},
				MergeOrder:          []string{"w0"},
			}, nil)
		if err == nil || resp.Verdict != core.VerdictFAIL {
			t.Fatalf("enforce merge failure = verdict %s err %v", resp.Verdict, err)
		}
		if resp.Signals["swarm.merged"] != false {
			t.Fatalf("swarm.merged signal = %#v", resp.Signals["swarm.merged"])
		}
	})

	t.Run("reader synthesis write failure", func(t *testing.T) {
		d := New(&fakeInner{name: "scout"}, &fakeBridge{}, swarm.ModeReader)
		wsFile := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		resp, err := d.enforce(context.Background(), reqWith(wsFile, nil),
			swarm.SwarmPlan{Mode: swarm.ModeReader},
			swarm.SwarmResult{Workers: []swarm.WorkerResult{{WorkerID: "w0", ExitCode: 0, ArtifactPath: filepath.Join(t.TempDir(), "missing.md")}}},
			nil)
		if err == nil || resp.Verdict != core.VerdictFAIL {
			t.Fatalf("reader synthesis write failure = verdict %s err %v", resp.Verdict, err)
		}
	})
}

func TestSwarmRunner_CoverageBridgeLauncherError(t *testing.T) {
	_, err := (bridgeLauncher{bridge: &errBridge{}}).Launch(context.Background(), swarm.LaunchRequest{})
	if err == nil || !strings.Contains(err.Error(), "transport down") {
		t.Fatalf("Launch error = %v", err)
	}
}
