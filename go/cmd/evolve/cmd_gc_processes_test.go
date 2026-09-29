package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func spawnOrphanIn(t *testing.T, dir string) int {
	t.Helper()
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof is not installed")
	}
	out, err := exec.Command("sh", "-c", "cd '"+dir+"' || exit 1; sleep 120 >/dev/null 2>&1 & echo $!").Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		ppid, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
		if err == nil && strings.TrimSpace(string(ppid)) == "1" {
			return pid
		}
		if time.Now().After(deadline) {
			t.Skip("orphans are not reparented to pid 1 on this host (subreaper)")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func gcClosedOutCycleTree(t *testing.T, projectRoot string, cycle int) string {
	t.Helper()
	tree := filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-cd3ae73e-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dossier.CyclesDir(projectRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dossier.CyclesDir(projectRoot), "cycle-"+strconv.Itoa(cycle)+".json")
	if err := os.WriteFile(name, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return tree
}

func processAlive(pid int) bool {
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

func TestRunGC_TerminatesAnOrphanLeftRunningInAClosedOutCycleTree(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	pid := spawnOrphanIn(t, gcClosedOutCycleTree(t, projectRoot, 1700))
	label := "pid=" + strconv.Itoa(pid)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "WOULD-TERMINATE "+label) {
		t.Fatalf("--dry-run does not name the orphan %s:\n%s\nstderr=%s", label, stdout.String(), stderr.String())
	}
	if !processAlive(pid) {
		t.Fatalf("--dry-run signaled %s — a preview must terminate nothing", label)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "terminated "+label) {
		t.Errorf("the explicit run does not report terminating %s:\n%s\nstderr=%s", label, stdout.String(), stderr.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Errorf("orphan %s still runs after `evolve gc`", label)
	}
}
