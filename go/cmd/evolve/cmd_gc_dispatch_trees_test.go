package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

const (
	deadDispatch  = "01R/1835/build/p999n1"
	aliveDispatch = "01R/1836/build/p4242n1"
)

func taggedAt(root string, pid int, comm, id string, args ...string) proctree.Process {
	return proctree.Process{Pid: pid, Ppid: 1, Started: time.Now().Add(-72 * time.Hour), Comm: comm, Args: args,
		Env: map[string]string{ipcenv.DispatchIDKey: id, gc.ProjectRootEnvKey: root}}
}

func saveTreeOf(t *testing.T, root, id string, members ...proctree.Process) string {
	t.Helper()
	if err := proctree.SaveTree(proctree.TreeDir(root), id, proctree.Identities(members)); err != nil {
		t.Fatal(err)
	}
	return proctree.TreeFile(proctree.TreeDir(root), id)
}

func gcRunOver(t *testing.T, h *fakeGCProcessHost, root string, dry bool) (gcRun, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	withGCProcessHost(t, h)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), dry, &stdout, &stderr)
	return r, &stdout, &stderr
}

var treeLine = regexp.MustCompile(`(?m)^  (?:WOULD-)?REMOVE-TREE (\S+)`)

func treePaths(out string) []string {
	var paths []string
	for _, m := range treeLine.FindAllStringSubmatch(out, -1) {
		paths = append(paths, m[1])
	}
	return paths
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func TestGCDispatchProcesses_DryRunListsTheSameStaleTreeFilesThatApplyRemoves(t *testing.T) {
	build := func() (string, *fakeGCProcessHost, string, string) {
		root := t.TempDir()
		member := taggedAt(root, 30469, "node", deadDispatch, "node", "mcp.js")
		stale := saveTreeOf(t, root, deadDispatch, member)
		live := saveTreeOf(t, root, aliveDispatch, taggedAt(root, 31, "node", aliveDispatch))
		return root, &fakeGCProcessHost{table: []proctree.Process{member}}, stale, live
	}

	root, h, stale, live := build()
	r, dryOut, _ := gcRunOver(t, h, root, true)
	dryFailed := r.dispatchProcesses(root, 24)
	if dryFailed || len(h.sent) != 0 || !exists(stale) || !exists(live) {
		t.Fatalf("dry-run: failed=%v signals=%v stale-exists=%v live-exists=%v, want a clean preview that keeps every file", dryFailed, h.sent, exists(stale), exists(live))
	}

	root2, h2, stale2, live2 := build()
	r2, applyOut, _ := gcRunOver(t, h2, root2, false)
	applyFailed := r2.dispatchProcesses(root2, 24)

	rel := func(root string, paths []string) []string {
		var out []string
		for _, p := range paths {
			out = append(out, strings.TrimPrefix(p, root))
		}
		return out
	}
	dry, applied := rel(root, treePaths(dryOut.String())), rel(root2, treePaths(applyOut.String()))
	if !slices.Equal(dry, applied) || len(dry) != 1 || !strings.Contains(dryOut.String(), "WOULD-REMOVE-TREE") || !strings.Contains(applyOut.String(), "  REMOVE-TREE ") {
		t.Errorf("dry-run lists %v, apply lists %v, want the same one file", dry, applied)
	}
	if applyFailed || exists(stale2) || !exists(live2) {
		t.Errorf("apply: failed=%v stale-exists=%v live-exists=%v, want the stale file gone and the live one kept", applyFailed, exists(stale2), exists(live2))
	}
}

func TestGCDispatchProcesses_AMemberThatReturnsBetweenPlanAndApplyKeepsItsFile(t *testing.T) {
	root := t.TempDir()
	first := taggedAt(root, 30469, "node", deadDispatch, "node")
	late := taggedAt(root, 905, "node", deadDispatch, "node")
	file := saveTreeOf(t, root, deadDispatch, first, late)
	h := &fakeGCProcessHost{table: []proctree.Process{first}}
	var sent []int
	withGCProcessHost(t, h)
	gcInjectedProcessHost.signal = func(pid int, _ syscall.Signal) error { sent = append(sent, pid); return nil }
	gcInjectedProcessHost.list = func(context.Context) ([]proctree.Process, error) {
		self := proctree.Process{Pid: os.Getpid(), Ppid: os.Getppid()}
		if len(sent) > 0 {
			return []proctree.Process{self, late}, nil
		}
		return []proctree.Process{self, first}, nil
	}
	var stdout bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &bytes.Buffer{})

	r.dispatchProcesses(root, 24)

	if !exists(file) || strings.Contains(stdout.String(), "REMOVE-TREE") {
		t.Errorf("the tree file of a dispatch with a new live member was removed:\n%s", stdout.String())
	}
}

func TestGCDispatchProcesses_ATreeFileThatCannotBeRemovedFailsTheRun(t *testing.T) {
	cases := []struct {
		name   string
		remove error
		failed bool
		stderr string
	}{
		{"a refused removal", errors.New("permission denied"), true, "permission denied"},
		{"a file that is already gone", fs.ErrNotExist, false, ""},
	}
	for _, tc := range cases {
		root := t.TempDir()
		file := saveTreeOf(t, root, deadDispatch)
		h := &fakeGCProcessHost{}
		r, _, stderr := gcRunOver(t, h, root, false)
		r.remove = func(string) error { return tc.remove }

		failed := r.dispatchProcesses(root, 24)

		if failed != tc.failed || (tc.stderr != "" && !(strings.Contains(stderr.String(), file) && strings.Contains(stderr.String(), tc.stderr))) {
			t.Errorf("%s: failed=%v stderr=%q, want failed=%v naming %s", tc.name, failed, stderr.String(), tc.failed, file)
		}
	}
}

func TestGCDispatchProcesses_ACorruptTreeFileStaysWithAWarning(t *testing.T) {
	root := t.TempDir()
	dir := proctree.TreeDir(root)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, stdout, stderr := gcRunOver(t, &fakeGCProcessHost{}, root, false)

	failed := r.dispatchProcesses(root, 24)

	if failed || !exists(bad) || !strings.Contains(stderr.String(), "WARN") || !strings.Contains(stderr.String(), "bad.json") || strings.Contains(stdout.String(), "REMOVE-TREE") {
		t.Errorf("failed=%v exists=%v stderr=%q, want a WARN and the file kept", failed, exists(bad), stderr.String())
	}
}

func TestGCDispatchProcesses_AnUnreadableTreeDirOrAFailedRelistFailsTheRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proctree.TreeDir(root), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r, _, stderr := gcRunOver(t, &fakeGCProcessHost{}, root, false)
	if !r.dispatchProcesses(root, 24) || !strings.Contains(stderr.String(), "dispatch tree sweep skipped") {
		t.Errorf("unreadable tree dir: stderr=%q, want a failed run", stderr.String())
	}

	root2 := t.TempDir()
	saveTreeOf(t, root2, deadDispatch)
	calls := 0
	withGCProcessHost(t, &fakeGCProcessHost{})
	gcInjectedProcessHost.list = func(context.Context) ([]proctree.Process, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("ps: exit 1")
		}
		return nil, nil
	}
	var stderr2 bytes.Buffer
	r2 := newGCRun(context.Background(), false, &bytes.Buffer{}, &stderr2)
	if !r2.dispatchProcesses(root2, 24) || !strings.Contains(stderr2.String(), "dispatch tree sweep skipped: ps: exit 1") {
		t.Errorf("failed relist: stderr=%q, want a failed run", stderr2.String())
	}
}

func TestGCDispatchProcesses_ReportsEachSparedTaggedProcessAndSignalsNone(t *testing.T) {
	for _, dry := range []bool{true, false} {
		root := t.TempDir()
		table := []proctree.Process{
			taggedAt(root, 61, "/opt/homebrew/bin/tmux", deadDispatch, "tmux", "new-session"),
			taggedAt(root, 62, "/go/bin/gopls", deadDispatch, "gopls"),
			taggedAt(root, 63, "/usr/bin/ssh-agent", deadDispatch, "ssh-agent"),
			taggedAt(root, 64, "node", aliveDispatch, "node"),
		}
		h := &fakeGCProcessHost{table: table}
		r, stdout, _ := gcRunOver(t, h, root, dry)

		failed := r.dispatchProcesses(root, 24)

		out := stdout.String()
		for _, want := range []string{`REPORT-ONLY pid=61 rule=shared`, `REPORT-ONLY pid=62 rule=not-killable`, `REPORT-ONLY pid=63 rule=not-killable`} {
			if !strings.Contains(out, want) {
				t.Errorf("dry=%v: stdout lacks %q:\n%s", dry, want, out)
			}
		}
		if strings.Contains(out, "pid=64") || failed || len(h.sent) != 0 {
			t.Errorf("dry=%v: failed=%v signals=%v stdout=%q, want no signal, rc 0 and no line for a live owner", dry, failed, h.sent, out)
		}
	}
}

func TestGCDispatchProcesses_ATaggedSshAgentIsSparedWhileTheMCPServerIsStopped(t *testing.T) {
	root := t.TempDir()
	h := &fakeGCProcessHost{table: []proctree.Process{
		taggedAt(root, 30469, "node", deadDispatch, "node", "mcp.js"),
		taggedAt(root, 63, "/usr/bin/ssh-agent", deadDispatch, "ssh-agent"),
	}}
	r, stdout, _ := gcRunOver(t, h, root, false)

	r.dispatchProcesses(root, 24)

	if !slices.Equal(h.sent, []int{30469}) || !strings.Contains(stdout.String(), "REPORT-ONLY pid=63 rule=not-killable") {
		t.Errorf("signals=%v stdout=%q, want SIGTERM to 30469 only and a report for 63", h.sent, stdout.String())
	}
}

func TestGCDispatchProcesses_ALiveRecordedSharedHelperLosesItsFileThenIsReported(t *testing.T) {
	for _, dry := range []bool{true, false} {
		root := t.TempDir()
		chrome := taggedAt(root, 903, "Google Chrome", deadDispatch)
		file := saveTreeOf(t, root, deadDispatch, chrome)
		h := &fakeGCProcessHost{table: []proctree.Process{chrome}}
		r, stdout, _ := gcRunOver(t, h, root, dry)

		r.dispatchProcesses(root, 24)

		out := stdout.String()
		tree, report := strings.Index(out, "REMOVE-TREE"), strings.Index(out, "REPORT-ONLY pid=903 rule=shared")
		if report < 0 || tree < 0 || len(h.sent) != 0 || exists(file) == !dry {
			t.Errorf("dry=%v: file-exists=%v signals=%v stdout=%q, want the file listed (removed unless dry-run), 903 reported and unsignalled", dry, exists(file), h.sent, out)
		}
	}
}
