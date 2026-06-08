package redteamcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes ledger.jsonl (+ optional state.json) into a temp .evolve/ and
// returns the ledger + state paths.
func fixture(t *testing.T, ledger, state string) (ledgerPath, statePath string) {
	t.Helper()
	ev := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(ev, 0o755); err != nil {
		t.Fatal(err)
	}
	ledgerPath = filepath.Join(ev, "ledger.jsonl")
	statePath = filepath.Join(ev, "state.json")
	if ledger != "" {
		if err := os.WriteFile(ledgerPath, []byte(ledger), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if state != "" {
		if err := os.WriteFile(statePath, []byte(state), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ledgerPath, statePath
}

const (
	terminal = `{"cycle":5,"kind":"cycle_terminal","role":"orchestrator"}` + "\n"
	scoutE   = `{"cycle":5,"kind":"agent_subprocess","role":"scout","challenge_token":"tok-s"}` + "\n"
	buildE   = `{"cycle":5,"kind":"agent_subprocess","role":"builder","challenge_token":"tok-b"}` + "\n"
	auditE   = `{"cycle":5,"kind":"agent_subprocess","role":"auditor","challenge_token":"tok-a"}` + "\n"
)

func TestLedgerRoleCompleteness(t *testing.T) {
	// Clean: terminal cycle has scout + builder + auditor → PASS.
	lp, _ := fixture(t, terminal+scoutE+buildE+auditE, "")
	if skip, err := LedgerRoleCompleteness(lp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	// Attack: cycle completed (terminal) but the auditor was never invoked → FAIL.
	lp, _ = fixture(t, terminal+scoutE+buildE, "")
	if _, err := LedgerRoleCompleteness(lp); err == nil {
		t.Error("auditor-skipped: want err (FAIL), got nil")
	}
	// Absent ledger → SKIP.
	if skip, err := LedgerRoleCompleteness(filepath.Join(t.TempDir(), "nope.jsonl")); !skip || err != nil {
		t.Errorf("absent ledger: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
	// No terminal entry yet → SKIP.
	lp, _ = fixture(t, scoutE, "")
	if skip, err := LedgerRoleCompleteness(lp); !skip || err != nil {
		t.Errorf("no-terminal: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}

func TestNoBatchCycleJump(t *testing.T) {
	ledger := terminal + scoutE
	// Clean: state matches ledger max.
	lp, sp := fixture(t, ledger, `{"lastCycleNumber": 5}`)
	if skip, err := NoBatchCycleJump(lp, sp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	// Attack: state claims cycle 14 but the ledger's max cycle is 5 → FAIL.
	lp, sp = fixture(t, ledger, `{"lastCycleNumber": 14}`)
	if _, err := NoBatchCycleJump(lp, sp); err == nil {
		t.Error("batch-jump: want err (FAIL), got nil")
	}
	// Pretty-printed +1 slack is allowed (in-flight cycle).
	lp, sp = fixture(t, ledger, "{\n  \"lastCycleNumber\": 6\n}")
	if _, err := NoBatchCycleJump(lp, sp); err != nil {
		t.Errorf("+1 slack: want nil (PASS), got %v", err)
	}
	// Absent state → SKIP.
	lp, _ = fixture(t, ledger, "")
	if skip, err := NoBatchCycleJump(lp, filepath.Join(t.TempDir(), "nostate.json")); !skip || err != nil {
		t.Errorf("absent state: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}

// TestReadLedger_ScanErrorFailsLoud — a ledger line exceeding the scanner buffer
// (bufio.ErrTooLong) must surface as a hard error so a check FAILs loud rather
// than evaluating a silently-truncated ledger (which could hide a gaming
// signature past the cutoff).
func TestReadLedger_ScanErrorFailsLoud(t *testing.T) {
	// One JSON line > the 1MB buffer cap → scan error.
	huge := `{"cycle":5,"kind":"agent_subprocess","role":"scout","challenge_token":"` +
		strings.Repeat("x", 2*1024*1024) + `"}` + "\n"
	lp, _ := fixture(t, huge, "")
	if _, err := LedgerRoleCompleteness(lp); err == nil {
		t.Error("oversized ledger line must FAIL loud (scan error), got nil")
	}
}

func TestChallengeTokenIntegrity(t *testing.T) {
	// Clean: all entries carry a token → PASS.
	lp, _ := fixture(t, terminal+scoutE+buildE+auditE, "")
	if skip, err := ChallengeTokenIntegrity(lp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	// Attack: a forged entry with an empty challenge_token → FAIL.
	forged := `{"cycle":5,"kind":"agent_subprocess","role":"auditor","challenge_token":""}` + "\n"
	lp, _ = fixture(t, terminal+scoutE+buildE+forged, "")
	if _, err := ChallengeTokenIntegrity(lp); err == nil {
		t.Error("forged-token: want err (FAIL), got nil")
	}
	// No agent_subprocess entries for the terminal cycle → SKIP.
	lp, _ = fixture(t, terminal, "")
	if skip, err := ChallengeTokenIntegrity(lp); !skip || err != nil {
		t.Errorf("no-entries: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}
