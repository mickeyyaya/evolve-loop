//go:build integration

package proctree

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const helperEnv = "PROCTREE_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestExecLister_ReadsTheTagOfARealChildAndTheReaperStopsOnlyThatChild(t *testing.T) {
	id := "itest/1/proctree/p" + time.Now().Format("150405.000000")
	child := exec.Command(os.Args[0], "-test.run=^$", "proctree-helper-arg")
	child.Env = append(os.Environ(), helperEnv+"=1", ipcenv.DispatchIDKey+"="+id)
	if err := child.Start(); err != nil {
		t.Fatalf("start the helper child: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	t.Cleanup(func() { _ = child.Process.Kill() })

	table, err := ExecLister()(context.Background())
	if err != nil {
		t.Fatalf("ExecLister: %v", err)
	}
	found := Select(table, TaggedWith(id))
	if len(found) != 1 || found[0].Pid != child.Process.Pid || !slices.Contains(found[0].Args, "proctree-helper-arg") {
		t.Fatalf("tagged = %+v, want exactly the helper child %d with its arguments", found, child.Process.Pid)
	}
	if _, ok := found[0].Env["PROCTREE_TEST_HELPER"]; ok {
		t.Errorf("env keeps a key outside the EVOLVE_ namespace")
	}

	rep := Reaper{List: ExecLister(), Signal: syscall.Kill, Sleep: time.Sleep, Grace: 2 * time.Second, Protected: []int{os.Getpid(), os.Getppid()}}.Reap(context.Background(), TaggedWith(id))

	if len(rep.Terminated) != 1 || len(rep.Survivors) != 0 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v, want one terminated child and no survivor", rep)
	}
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Errorf("the helper child is still running after the reap")
	}
}
