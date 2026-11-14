package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadHandoffLedger_AMissingLedgerReportsTheStatError(t *testing.T) {
	t.Parallel()

	_, err := ReadHandoffLedger(filepath.Join(t.TempDir(), "absent.json"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing ledger = %v, want os.ErrNotExist", err)
	}
}

func TestReadHandoffLedger_ASymlinkedLedgerIsRefused(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "handoff-tdd-build.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHandoffLedger(link); !errors.Is(err, errHandoffLedgerNotRegular) {
		t.Errorf("a symlinked ledger = %v, want errHandoffLedgerNotRegular", err)
	}
}

func TestReadHandoffLedger_AnOversizeLedgerIsRefusedWithTheSizeError(t *testing.T) {
	t.Parallel()

	ledger := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	f, err := os.Create(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxHandoffLedgerBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHandoffLedger(ledger); !errors.Is(err, errHandoffLedgerTooLarge) {
		t.Errorf("an oversize ledger = %v, want errHandoffLedgerTooLarge", err)
	}
}
