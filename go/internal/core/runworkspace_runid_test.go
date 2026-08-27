package core

import (
	"os"
	"path/filepath"
	"testing"
)

// runworkspace_runid_test.go — RunIDFromWorkspace is the single resolver every
// OUT-OF-PROCESS agent_subprocess ledger writer uses to stamp run_id.
//
// Cycle-1571 follow-up (H1). PR #503 removed ship's cross-run binding fallback
// on the stated premise that "every current recorder stamps run_id". Three of
// the four agent_subprocess writers did not: subagent/run.go's hand-built JSON
// literal, subagent/subagent.go's LedgerEntry, and cyclesimulator's map. Only
// core/phase_bindings.go was stamped, and only because it routes through the
// Orchestrator's stampingLedger. The premise was checked empirically against
// recent ledger rows — all written by the one writer that DOES stamp — instead
// of structurally against every writer. Consequence: an auditor entry written
// by `evolve subagent run` (the sanctioned manual re-audit) can never match a
// run-scoped lookup, so ship hard-stops AUDIT_BINDING_NO_AUDITOR where it
// previously bound and shipped.
//
// The run workspace is already on hand at every one of those call sites, and
// its run.json already carries run_id (cyclestate.CycleState.RunID, mirrored by
// adapters/storage.mirrorRunState).

func TestRunIDFromWorkspace_ReadsRunJSON(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// Shape copied from a real file: .evolve/runs/cycle-1519/run.json.
	mustWriteRunState(t, ws, `{"cycle_id":1519,"phase":"aborted","run_id":"01M09657TDN6Q1VMJK1XKYR376"}`)

	if got := RunIDFromWorkspace(ws); got != "01M09657TDN6Q1VMJK1XKYR376" {
		t.Errorf("RunIDFromWorkspace = %q, want the run.json run_id", got)
	}
}

// TestRunIDFromWorkspace_UnresolvableIsEmpty: the read is fail-SOFT (an
// unresolvable id yields "", and the writer then OMITS the key rather than
// stamping an empty identity). The fail-CLOSED half lives in the consumer:
// ship refuses to bind an entry that carries no run_id. Inventing or
// zero-filling an identity here would defeat that.
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
