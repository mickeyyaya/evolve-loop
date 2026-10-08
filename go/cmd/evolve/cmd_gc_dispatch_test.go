package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

type fakeGCProcessHost struct {
	table []proctree.Process
	lists int
	sent  []int
}

func (h *fakeGCProcessHost) host() *gcProcessHost {
	return &gcProcessHost{
		list: func(context.Context) ([]proctree.Process, error) {
			h.lists++
			self := proctree.Process{Pid: os.Getpid(), Ppid: os.Getppid()}
			if len(h.sent) > 0 {
				return []proctree.Process{self}, nil
			}
			return append(slices.Clone(h.table), self), nil
		},
		signal: func(pid int, _ syscall.Signal) error { h.sent = append(h.sent, pid); return nil },
		sleep:  func(time.Duration) {},
		alive:  func(pid int) bool { return pid == 4242 },
	}
}

func gcDispatchTable(root string) []proctree.Process {
	old := time.Now().Add(-72 * time.Hour)
	return []proctree.Process{
		{Pid: 30469, Ppid: 1, Started: old, Comm: "node", Env: map[string]string{ipcenv.DispatchIDKey: "01R/1835/build/p999n1", gc.ProjectRootEnvKey: root}},
		{Pid: 30470, Ppid: 1, Started: old, Comm: "node", Env: map[string]string{ipcenv.DispatchIDKey: "01R/1836/build/p4242n1", gc.ProjectRootEnvKey: root}},
		{Pid: 777, Ppid: 1, Started: old, Comm: "/usr/bin/tail", Args: []string{"tail", "-F", root + "/.evolve/boundary-loop.log"}},
		{Pid: 901, Ppid: 1, Started: old, Comm: "Google Chrome Helper", Env: map[string]string{}},
		{Pid: 902, Ppid: 1, Started: old, Comm: "claude", Env: map[string]string{gc.ProjectRootEnvKey: root}},
		{Pid: 903, Ppid: 1, Started: old, Comm: "Google Chrome", Env: map[string]string{ipcenv.DispatchIDKey: "01R/1835/build/p999n1", gc.ProjectRootEnvKey: root}},
	}
}

func withGCProcessHost(t *testing.T, h *fakeGCProcessHost) {
	t.Helper()
	orig := gcInjectedProcessHost
	t.Cleanup(func() { gcInjectedProcessHost = orig })
	gcInjectedProcessHost = h.host()
}

func TestGCDispatchProcesses_DryRunListsAndSignalsNothing(t *testing.T) {
	root := t.TempDir()
	h := &fakeGCProcessHost{table: gcDispatchTable(root)}
	withGCProcessHost(t, h)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)

	failed := r.dispatchProcesses(root, 24)

	out := stdout.String()
	for _, want := range []string{"2 dispatch process(es) would be stopped", "WOULD-STOP pid=30469 rule=stale-dispatch", `WOULD-STOP pid=777 rule=log-tail comm="/usr/bin/tail" args=["tail" "-F" "` + root + `/.evolve/boundary-loop.log"]`} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	for _, foreign := range []string{"pid=30470", "pid=901", "pid=902", "pid=903"} {
		if strings.Contains(out, foreign) {
			t.Errorf("stdout names %s, a process without the proof:\n%s", foreign, out)
		}
	}
	if failed || len(h.sent) != 0 {
		t.Errorf("failed=%v signals=%v, want a clean preview that signals nothing", failed, h.sent)
	}
}

func TestGCDispatchProcesses_ApplyStopsOnlyTheProvenProcesses(t *testing.T) {
	root := t.TempDir()
	h := &fakeGCProcessHost{table: gcDispatchTable(root)}
	withGCProcessHost(t, h)
	var stdout bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, io.Discard)

	failed := r.dispatchProcesses(root, 24)

	if failed || len(h.sent) != 2 || h.sent[0] != 30469 || h.sent[1] != 777 {
		t.Errorf("failed=%v signals=%v, want SIGTERM to 30469 and 777 only", failed, h.sent)
	}
	if !strings.Contains(stdout.String(), "stopped 2 dispatch process(es)") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestGCDispatchProcesses_ReadsThePersistedTreeOfADeadDispatch(t *testing.T) {
	root := t.TempDir()
	gcClosedOutCycleTree(t, root, 1700)
	table := gcDispatchTable(root)
	chrome := table[len(table)-1]
	orphan := proctree.Process{Pid: 904, Ppid: 1, Started: chrome.Started, Comm: "node", Env: map[string]string{ipcenv.DispatchIDKey: "01R/1700/build/p4242n1", gc.ProjectRootEnvKey: root}}
	table = append(table, orphan)
	if err := proctree.SaveTree(proctree.TreeDir(root), "01R/1835/build/p999n1", []proctree.Identity{chrome.Identity()}); err != nil {
		t.Fatal(err)
	}
	if err := proctree.SaveTree(proctree.TreeDir(root), "01R/1700/build/p4242n1", []proctree.Identity{orphan.Identity()}); err != nil {
		t.Fatal(err)
	}
	h := &fakeGCProcessHost{table: table}
	withGCProcessHost(t, h)
	var stdout bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, io.Discard)

	r.dispatchProcesses(root, 24)

	if !strings.Contains(stdout.String(), "WOULD-STOP pid=904 rule=stale-dispatch") {
		t.Errorf("stdout = %q, want the orphan in the persisted tree of the closed cycle", stdout.String())
	}
	if strings.Contains(stdout.String(), "pid=903") {
		t.Errorf("stdout = %q: a shared helper is never stopped by persisted-tree membership alone", stdout.String())
	}
}
