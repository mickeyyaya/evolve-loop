package guardcmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	boundaryDeliverable = "/repo/boundary/self-heal-report.md"
	boundaryProtected   = "/repo/go/internal/guards/role.go"
)

func abortedCycleFixture(t *testing.T) (evolveDir, workspace string) {
	t.Helper()
	root := t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	workspace = filepath.Join(evolveDir, "runs", "cycle-30")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 30, Phase: "aborted", WorkspacePath: workspace}
	if err := storage.New(evolveDir).WriteCycleState(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	return evolveDir, workspace
}

func runRoleWrite(evolveDir, path string) int {
	input := `{"tool_name":"Write","tool_input":{"file_path":"` + path + `"}}`
	var stdout, stderr bytes.Buffer
	return RunGuard([]string{"--evolve-dir", evolveDir, "role"}, strings.NewReader(input), &stdout, &stderr)
}

func TestRunGuard_Role_AbortedCycleWithNoLeaseLetsTheBoundarySelfHealWrite(t *testing.T) {
	t.Parallel()
	evolveDir, _ := abortedCycleFixture(t)
	if rc := runRoleWrite(evolveDir, boundaryDeliverable); rc != 0 {
		t.Fatalf("aborted cycle with no lease: exit %d, want 0 (the self-heal agent must write its deliverable)", rc)
	}
}

func TestRunGuard_Role_AbortedCycleWithALiveOwnerStillRefuses(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	lease := runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}
	if err := runlease.Write(workspace, lease, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryDeliverable); rc != 2 {
		t.Fatalf("live owner on an aborted cycle: exit %d, want 2", rc)
	}
}

func TestRunGuard_Role_AbortedCycleWithAnUnreadableLeaseStillRefuses(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	if err := os.WriteFile(runlease.PathIn(workspace), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryDeliverable); rc != 2 {
		t.Fatalf("unreadable lease: exit %d, want 2", rc)
	}
}

func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	return pid
}

func TestRunGuard_Role_AbortedCycleWithAFreshLeaseAndAnExitedOwnerLetsTheBoundarySelfHealWrite(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	if err := runlease.Write(workspace, runlease.Lease{RunID: "r", OwnerPID: exitedPID(t)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryDeliverable); rc != 0 {
		t.Fatalf("fresh lease, exited owner: exit %d, want 0", rc)
	}
}

func TestRunGuard_Role_AbortedCycleWithAStaleLeaseAndARunningPIDLetsTheBoundarySelfHealWrite(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	at := time.Now().Add(-runlease.DefaultTTL - time.Minute)
	if err := runlease.Write(workspace, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, at); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryDeliverable); rc != 0 {
		t.Fatalf("stale lease, running pid: exit %d, want 0", rc)
	}
}

func TestRunGuard_Role_DeadOwnerOnAnAbortedCycleAllowsAProtectedWrite(t *testing.T) {
	t.Parallel()
	evolveDir, _ := abortedCycleFixture(t)
	if rc := runRoleWrite(evolveDir, boundaryProtected); rc != 0 {
		t.Fatalf("dead owner, protected path: exit %d, want 0", rc)
	}
}

func TestRunGuard_Role_LiveOwnerOnAnAbortedCycleRefusesAProtectedWrite(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	if err := runlease.Write(workspace, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryProtected); rc != 2 {
		t.Fatalf("live owner, protected path: exit %d, want 2", rc)
	}
}

func TestRunGuard_Role_UnreadableLeaseOnAnAbortedCycleRefusesAProtectedWrite(t *testing.T) {
	t.Parallel()
	evolveDir, workspace := abortedCycleFixture(t)
	if err := os.WriteFile(runlease.PathIn(workspace), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc := runRoleWrite(evolveDir, boundaryProtected); rc != 2 {
		t.Fatalf("unreadable lease, protected path: exit %d, want 2", rc)
	}
}
