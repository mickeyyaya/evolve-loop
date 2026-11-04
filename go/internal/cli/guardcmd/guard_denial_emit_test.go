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

func runRoleHook(evolveDir, path string) (int, string) {
	stdin := strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"` + path + `","content":"x"},"cwd":"/wt"}`)
	var stdout, stderr bytes.Buffer
	rc := RunGuard([]string{"role", "--evolve-dir", evolveDir}, stdin, &stdout, &stderr)
	return rc, stderr.String()
}

func TestRunGuard_ADenialInACellEmitsGuardDeniedForThatDispatch(t *testing.T) {
	root, evolveDir := denialCell(t)

	rc, stderr := runRoleHook(evolveDir, "/elsewhere/internal/x.go")

	if rc != 2 {
		t.Fatalf("rc=%d stderr=%s: the role guard must deny a write outside the workspace", rc, stderr)
	}
	d, found := guards.DenialOf(root, cellDispatch)
	if !found || d.Guard != "role" || d.Path != "/elsewhere/internal/x.go" || !strings.Contains(d.Reason, "may not write outside workspace") {
		t.Fatalf("denial %+v found %v: the hook must record GUARD_DENIED for the dispatch", d, found)
	}
}

func TestRunGuard_AnAllowRecordsNoDenial(t *testing.T) {
	root, evolveDir := denialCell(t)

	rc, _ := runRoleHook(evolveDir, filepath.Join(paths.RunWorkspace(root, 7), "build-report.md"))

	if rc != 0 {
		t.Fatalf("rc=%d: a write inside the workspace passes", rc)
	}
	if _, found := guards.DenialOf(root, cellDispatch); found {
		t.Fatal("an allowed call must record no denial")
	}
}

func TestRunGuard_WithNoProjectRootMarkerTheDenialGoesBesideTheEvolveDir(t *testing.T) {
	root, evolveDir := denialCell(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", "")

	if rc, _ := runRoleHook(evolveDir, "/elsewhere/x.go"); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if _, found := guards.DenialOf(root, cellDispatch); !found {
		t.Fatal("the project root defaults to the parent of the evolve dir")
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
