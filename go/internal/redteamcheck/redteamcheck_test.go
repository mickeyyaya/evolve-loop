package redteamcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	lp, _ := fixture(t, terminal+scoutE+buildE+auditE, "")
	if skip, err := LedgerRoleCompleteness(lp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	lp, _ = fixture(t, terminal+scoutE+buildE, "")
	if _, err := LedgerRoleCompleteness(lp); err == nil {
		t.Error("auditor-skipped: want err (FAIL), got nil")
	}
	if skip, err := LedgerRoleCompleteness(filepath.Join(t.TempDir(), "nope.jsonl")); !skip || err != nil {
		t.Errorf("absent ledger: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
	lp, _ = fixture(t, scoutE, "")
	if skip, err := LedgerRoleCompleteness(lp); !skip || err != nil {
		t.Errorf("no-terminal: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}

func TestNoBatchCycleJump(t *testing.T) {
	ledger := terminal + scoutE
	lp, sp := fixture(t, ledger, `{"lastCycleNumber": 5}`)
	if skip, err := NoBatchCycleJump(lp, sp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	lp, sp = fixture(t, ledger, `{"lastCycleNumber": 14}`)
	if _, err := NoBatchCycleJump(lp, sp); err == nil {
		t.Error("batch-jump: want err (FAIL), got nil")
	}
	lp, sp = fixture(t, ledger, "{\n  \"lastCycleNumber\": 6\n}")
	if _, err := NoBatchCycleJump(lp, sp); err != nil {
		t.Errorf("+1 slack: want nil (PASS), got %v", err)
	}
	lp, _ = fixture(t, ledger, "")
	if skip, err := NoBatchCycleJump(lp, filepath.Join(t.TempDir(), "nostate.json")); !skip || err != nil {
		t.Errorf("absent state: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}

func TestReadLedger_ScanErrorFailsLoud(t *testing.T) {
	huge := `{"cycle":5,"kind":"agent_subprocess","role":"scout","challenge_token":"` +
		strings.Repeat("x", 2*1024*1024) + `"}` + "\n"
	lp, _ := fixture(t, huge, "")
	if _, err := LedgerRoleCompleteness(lp); err == nil {
		t.Error("oversized ledger line must FAIL loud (scan error), got nil")
	}
}

func TestChallengeTokenIntegrity(t *testing.T) {
	lp, _ := fixture(t, terminal+scoutE+buildE+auditE, "")
	if skip, err := ChallengeTokenIntegrity(lp); skip || err != nil {
		t.Errorf("clean: skip=%v err=%v, want false/nil (PASS)", skip, err)
	}
	forged := `{"cycle":5,"kind":"agent_subprocess","role":"auditor","challenge_token":""}` + "\n"
	lp, _ = fixture(t, terminal+scoutE+buildE+forged, "")
	if _, err := ChallengeTokenIntegrity(lp); err == nil {
		t.Error("forged-token: want err (FAIL), got nil")
	}
	lp, _ = fixture(t, terminal, "")
	if skip, err := ChallengeTokenIntegrity(lp); !skip || err != nil {
		t.Errorf("no-entries: skip=%v err=%v, want true/nil (SKIP)", skip, err)
	}
}
