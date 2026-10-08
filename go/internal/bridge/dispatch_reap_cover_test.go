package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

type failingPaneTmux struct {
	fakeTmux
	err error
}

func (f *failingPaneTmux) PanePIDs(context.Context, string) ([]int, error) { return nil, f.err }

func failingList(err error) func(context.Context) ([]proctree.Process, error) {
	return func(context.Context) ([]proctree.Process, error) { return nil, err }
}

func TestRecordPane_AnUnreadablePaneWarnsAndRecordsNothing(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	listed := 0
	deps := Deps{Tmux: &failingPaneTmux{err: errors.New("no server running")}, Stderr: &stderr,
		ListProcesses: func(context.Context) ([]proctree.Process, error) { listed++; return nil, nil }}
	s := &dispatchSweep{id: thisDispatch}

	s.recordPane(context.Background(), deps, "s1")

	if !strings.Contains(stderr.String(), "[bridge] WARN dispatch "+thisDispatch+": cannot read the pane pids of session s1: no server running") || listed != 0 || s.recorded != nil {
		t.Errorf("stderr=%q listed=%d recorded=%v, want the pane WARN, no listing and nothing recorded", stderr.String(), listed, s.recorded)
	}
}

func TestRecordPane_AFailedListingWarnsAndRecordsNothing(t *testing.T) {
	t.Parallel()
	var events []string
	var stderr bytes.Buffer
	deps := Deps{Tmux: &panePIDTmux{panePIDs: []int{500}, events: &events}, Stderr: &stderr, ListProcesses: failingList(errors.New("ps: exit 1"))}
	s := &dispatchSweep{id: thisDispatch}

	s.recordPane(context.Background(), deps, "s1")

	if !strings.Contains(stderr.String(), "cannot record the pane tree of session s1: ps: exit 1") || s.recorded != nil || s.panes != nil {
		t.Errorf("stderr=%q recorded=%v panes=%v, want the listing WARN and nothing recorded", stderr.String(), s.recorded, s.panes)
	}
}

func TestRecordPane_AnUnsavableTreeWarnsAndKeepsTheTreeInMemory(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var events []string
	var stderr bytes.Buffer
	h := &hostTable{tables: [][]proctree.Process{{hostProc(500, 1, ""), hostProc(501, 500, "")}}, events: &events}
	deps := reapDeps(h, &panePIDTmux{panePIDs: []int{500}, events: &events}, &stderr, nil)
	s := &dispatchSweep{id: thisDispatch, treeDir: filepath.Join(file, "dispatch-trees")}

	s.recordPane(context.Background(), deps, "s1")

	if !strings.Contains(stderr.String(), "cannot persist the pane tree for gc: make dispatch tree dir") || len(s.recorded) != 2 {
		t.Errorf("stderr=%q recorded=%v, want the persist WARN and both pane processes kept for the end sweep", stderr.String(), s.recorded)
	}
}

func TestFinish_AnUnremovableTreeFileWarnsAfterACleanSweep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := proctree.TreeFile(dir, thisDispatch)
	if err := os.MkdirAll(filepath.Join(path, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	var events []string
	var stderr bytes.Buffer
	h := &hostTable{tables: [][]proctree.Process{nil}, events: &events}
	s := &dispatchSweep{id: thisDispatch, treeDir: dir}

	s.finish(context.Background(), reapDeps(h, &fakeTmux{}, &stderr, nil))

	if !strings.Contains(stderr.String(), "[bridge] WARN dispatch "+thisDispatch+": remove dispatch tree: ") || len(h.sent) != 0 {
		t.Errorf("stderr=%q signalled=%v, want the remove WARN and no signal", stderr.String(), h.sent)
	}
}

func TestLivePaneTree_AFailedListingSparesEverySharedHelper(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	s := &dispatchSweep{id: thisDispatch, panes: []proctree.Identity{{Pid: 500, Started: reapT0}}}

	got := s.livePaneTree(context.Background(), Deps{Stderr: &stderr, ListProcesses: failingList(errors.New("ps: exit 1"))})

	if got != nil || !strings.Contains(stderr.String(), "cannot list the live pane tree, so each shared helper is spared: ps: exit 1") {
		t.Errorf("livePaneTree = %v, stderr=%q, want no tree and the WARN", got, stderr.String())
	}
}

func TestExecTmuxPanePIDs_AMissingSessionIsAnError(t *testing.T) {
	t.Parallel()
	got, err := execTmux{}.PanePIDs(context.Background(), "evolve-cover-no-such-session-"+thisDispatch[:5])

	if err == nil || !strings.HasPrefix(err.Error(), "tmux list-panes: ") || got != nil {
		t.Errorf("PanePIDs = %v, %v, want no pids and a tmux list-panes error", got, err)
	}
}
