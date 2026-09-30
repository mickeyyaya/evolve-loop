package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunIDFromWorkspace_ReadsRunJSON(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// Shape copied from a real run.json file.
	mustWriteRunState(t, ws, `{"cycle_id":1519,"phase":"aborted","run_id":"01M09657TDN6Q1VMJK1XKYR376"}`)

	if got := RunIDFromWorkspace(ws); got != "01M09657TDN6Q1VMJK1XKYR376" {
		t.Errorf("RunIDFromWorkspace = %q, want the run.json run_id", got)
	}
}

func TestRunIDFromWorkspace_UnresolvableIsEmpty(t *testing.T) {
	t.Parallel()
	noFile := t.TempDir()

	malformed := t.TempDir()
	mustWriteRunState(t, malformed, `{not json`)

	noKey := t.TempDir()
	mustWriteRunState(t, noKey, `{"cycle_id":7}`)

	for name, ws := range map[string]string{
		"run.json absent":     noFile,
		"run.json malformed":  malformed,
		"run.json has no key": noKey,
	} {
		if got := RunIDFromWorkspace(ws); got != "" {
			t.Errorf("%s: RunIDFromWorkspace = %q, want \"\" (fail-soft read)", name, got)
		}
	}
}

// TestRunIDFromWorkspace_EmptyWorkspace: a caller with no workspace at all
// (standalone `evolve subagent run` outside a cycle) must not panic or read a
// stray ./run.json from the process cwd.
func TestRunIDFromWorkspace_EmptyWorkspace(t *testing.T) {
	t.Parallel()
	if got := RunIDFromWorkspace(""); got != "" {
		t.Errorf("RunIDFromWorkspace(\"\") = %q, want \"\"", got)
	}
}

func mustWriteRunState(t *testing.T, ws, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, RunStateFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
