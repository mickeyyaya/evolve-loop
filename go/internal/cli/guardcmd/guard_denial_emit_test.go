package guardcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const cellDispatch = "run-41/7/build/p4242n1t9"

func denialCell(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = paths.EvolveDirOf(root)
	ws := paths.RunWorkspace(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"cycle_id":7,"phase":"build","workspace_path":"` + ws + `","run_id":"run-41"}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ipcenv.DispatchIDKey, cellDispatch)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	return root, evolveDir
}

type emitted struct {
	root   string
	events []signalcenter.Event
}

func (e *emitted) guard() Guard {
	return Guard{Emit: func(root string, ev signalcenter.Event) { e.root, e.events = root, append(e.events, ev) }}
}

func (e *emitted) denial() (guards.Denial, bool) {
	if len(e.events) != 1 || e.events[0].Code != guards.CodeDenied || e.events[0].Fields["dispatch_id"] != cellDispatch {
		return guards.Denial{}, false
	}
	ev := e.events[0]
	return guards.Denial{Guard: ev.Fields["guard"], Path: ev.Fields["path"], Reason: ev.Reason}, true
}

func runRoleHook(g Guard, evolveDir, path string) (int, string) {
	stdin := strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"` + path + `","content":"x"},"cwd":"/wt"}`)
	var stdout, stderr bytes.Buffer
	rc := g.Run([]string{"role", "--evolve-dir", evolveDir}, stdin, &stdout, &stderr)
	return rc, stderr.String()
}

func TestRunGuard_ADenialInACellEmitsGuardDeniedForThatDispatch(t *testing.T) {
	root, evolveDir := denialCell(t)
	out := &emitted{}

	rc, stderr := runRoleHook(out.guard(), evolveDir, "/elsewhere/internal/x.go")

	if rc != 2 || out.root != root {
		t.Fatalf("rc=%d root=%q stderr=%s: the role guard must deny a write outside the workspace and emit for the cell root", rc, out.root, stderr)
	}
	d, found := out.denial()
	if !found || d.Guard != "role" || d.Path != "/elsewhere/internal/x.go" || !strings.Contains(d.Reason, "may not write outside workspace") {
		t.Fatalf("denial %+v found %v: the hook must record GUARD_DENIED for the dispatch", d, found)
	}
}

func TestRunGuard_AnAllowRecordsNoDenial(t *testing.T) {
	root, evolveDir := denialCell(t)
	out := &emitted{}

	rc, _ := runRoleHook(out.guard(), evolveDir, filepath.Join(paths.RunWorkspace(root, 7), "build-report.md"))

	if rc != 0 {
		t.Fatalf("rc=%d: a write inside the workspace passes", rc)
	}
	if len(out.events) != 0 {
		t.Fatal("an allowed call must record no denial")
	}
}

func TestRunGuard_WithNoProjectRootMarkerTheDenialGoesBesideTheEvolveDir(t *testing.T) {
	root, evolveDir := denialCell(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	out := &emitted{}

	if rc, _ := runRoleHook(out.guard(), evolveDir, "/elsewhere/x.go"); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if out.root != root {
		t.Fatalf("root %q: the project root defaults to the parent of the evolve dir", out.root)
	}
}

func TestRunGuard_ADenialOutsideACellOrWithNoEmitterEmitsNothing(t *testing.T) {
	_, evolveDir := denialCell(t)
	if rc, _ := runRoleHook(Guard{}, evolveDir, "/elsewhere/x.go"); rc != 2 {
		t.Fatalf("rc=%d: RunGuard with no emitter still denies", rc)
	}
	t.Setenv(ipcenv.DispatchIDKey, "")
	out := &emitted{}
	if rc, _ := runRoleHook(out.guard(), evolveDir, "/elsewhere/x.go"); rc != 2 || len(out.events) != 0 {
		t.Fatalf("rc=%d events=%v: outside a cell the deny emits nothing", rc, out.events)
	}
}

func TestRunGuard_ABadFlagBeforeOrAfterTheGuardNameIsExitTen(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--no-such-flag"}, {"role", "--no-such-flag"}} {
		var stdout, stderr bytes.Buffer
		if rc := RunGuard(args, strings.NewReader(""), &stdout, &stderr); rc != 10 {
			t.Errorf("RunGuard(%v) = %d, want 10", args, rc)
		}
	}
}
