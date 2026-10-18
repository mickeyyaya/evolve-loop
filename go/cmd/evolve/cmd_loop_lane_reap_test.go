package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func writeLanePin(t *testing.T, root string, cycle int, todo string) {
	t.Helper()
	dir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pin, _ := json.Marshal(map[string]any{"todo_ids": []string{todo}})
	if err := os.WriteFile(filepath.Join(dir, committedset.LanePinFile), pin, 0o644); err != nil {
		t.Fatal(err)
	}
}

func taggedProcess(pid int, id, root string) proctree.Process {
	return proctree.Process{Pid: pid, Ppid: 1, Comm: "node", Env: map[string]string{ipcenv.DispatchIDKey: id, gc.ProjectRootEnvKey: root}}
}

type reapCalls struct{ ids [][]string }

func (r *reapCalls) reap(_ context.Context, ids []string) proctree.Report {
	r.ids = append(r.ids, ids)
	return proctree.Report{Terminated: []proctree.Process{{Pid: 1}}, Errors: []string{"refuse to signal protected pid 1"}}
}

func stoppedLane() orchestrator.Lane {
	return orchestrator.Lane{Key: "A", Scope: []string{"A"}, Started: time.Now().Add(-time.Hour)}
}

func TestReapLane_AStoppedLaneReapsOnceWithItsOwnDispatchIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLanePin(t, root, 1835, "A")
	table := []proctree.Process{
		taggedProcess(10, "01R/1835/build/p999n2", root),
		taggedProcess(11, "01R/1835/build/p999n1", root),
		taggedProcess(12, "01R/1835/build/p999n1", root),
		taggedProcess(13, "01R/1836/build/p998n1", root),
		taggedProcess(14, "01R/1835/build/p997n1", "/another/plane"),
		{Pid: 15, Ppid: 1, Comm: "node", Env: map[string]string{ipcenv.DispatchIDKey: "not-an-id", gc.ProjectRootEnvKey: root}},
	}
	list := func(context.Context) ([]proctree.Process, error) { return table, nil }
	calls := &reapCalls{}
	var warn bytes.Buffer

	reapLane(context.Background(), root, stoppedLane(), list, calls.reap, &warn)

	want := [][]string{{"01R/1835/build/p999n1", "01R/1835/build/p999n2"}}
	if !reflect.DeepEqual(calls.ids, want) {
		t.Errorf("reaps = %v, want exactly one with the two dispatches of cycle 1835 on this plane", calls.ids)
	}
	if !strings.Contains(warn.String(), "stopped lane A (cycle 1835) stopped 1 process(es) of 2 dispatch(es)") || !strings.Contains(warn.String(), "refuse to signal protected pid 1") {
		t.Errorf("stderr = %q, want the reap line that names the lane and its cycle, and each reap error", warn.String())
	}
}

func TestReapLane_ReapsNothingWithNoCycleNoDispatchOrNoTable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeLanePin(t, root, 1840, "A")
	empty := func(context.Context) ([]proctree.Process, error) { return nil, nil }
	broken := func(context.Context) ([]proctree.Process, error) { return nil, errors.New("ps failed") }
	cases := []struct {
		name string
		root string
		list proctree.Lister
		warn string
	}{
		{"no cycle holds the lane", t.TempDir(), empty, "no cycle holds the stopped lane A"},
		{"the cycle has no dispatch", root, empty, ""},
		{"the process table does not list", root, broken, "ps failed"},
	}
	for _, tc := range cases {
		calls := &reapCalls{}
		var warn bytes.Buffer
		reapLane(context.Background(), tc.root, stoppedLane(), tc.list, calls.reap, &warn)
		if len(calls.ids) != 0 {
			t.Errorf("%s: reaps = %v, want none", tc.name, calls.ids)
		}
		if !strings.Contains(warn.String(), tc.warn) || (tc.warn == "" && warn.Len() != 0) {
			t.Errorf("%s: stderr = %q, want %q", tc.name, warn.String(), tc.warn)
		}
	}
}

func TestReapDispatches_SignalsOnlyTheStaleProcessesOfTheGivenDispatches(t *testing.T) {
	root := t.TempDir()
	table := append(gcDispatchTable(root), taggedProcess(30471, "01R/1837/build/p998n1", root))
	h := &fakeGCProcessHost{table: table}
	withGCProcessHost(t, h)
	var stdout, stderr bytes.Buffer

	rep := newGCRun(context.Background(), false, &stdout, &stderr).reapDispatches(root, []string{"01R/1835/build/p999n1", "01R/1836/build/p4242n1"})

	if !slices.Equal(h.sent, []int{30469}) {
		t.Errorf("signalled = %v, want only 30469: 30470 has a live owner, 903 is a shared helper, 30471 and the tail are out of scope", h.sent)
	}
	if len(rep.Terminated) != 1 || len(rep.Errors) != 0 {
		t.Errorf("report = %+v, want one terminated process and no error", rep)
	}
}

func TestLaneReap_TheCoordinatorPortReportsALaneWithNoCycle(t *testing.T) {
	t.Parallel()
	b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	b.laneReap()(stoppedLane())
	if !strings.Contains(console.String(), "no cycle holds the stopped lane A") {
		t.Errorf("console = %q, want the WARN of a lane with no cycle", console.String())
	}
}

func TestLaneReap_TheCoordinatorPortReapsTheStoppedLanesDispatchesThroughTheGCSweep(t *testing.T) {
	root := t.TempDir()
	b, console := iscsCoordinator(root, t.TempDir(), nil)
	writeLanePin(t, b.cfg.ProjectRoot, 1835, "A")
	h := &fakeGCProcessHost{table: []proctree.Process{taggedProcess(30469, "01R/1835/build/p999n1", b.cfg.ProjectRoot), taggedProcess(30480, "01R/1836/build/p999n1", b.cfg.ProjectRoot)}}
	withGCProcessHost(t, h)

	b.laneReap()(stoppedLane())

	if !slices.Equal(h.sent, []int{30469}) {
		t.Errorf("signalled = %v, want only 30469, the dispatch of the stopped lane's cycle 1835", h.sent)
	}
	if !strings.Contains(console.String(), "stopped lane A (cycle 1835) stopped 1 process(es) of 1 dispatch(es)") {
		t.Errorf("console = %q, want the reap line", console.String())
	}
}
