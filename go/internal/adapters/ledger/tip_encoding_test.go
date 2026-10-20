package ledger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ledgertip"
)

func TestAppendLineAndReplaceTip_ATipRenameFailureIsNamedAndLeavesNoTmp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := New(dir)
	if err := os.MkdirAll(filepath.Join(l.tipPath, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := l.appendLineAndReplaceTip(map[string]int{"entry_seq": 0}, 0)

	if err == nil || !strings.Contains(err.Error(), "tip rename") {
		t.Fatalf("err = %v, want a tip rename failure", err)
	}
	if matches, _ := filepath.Glob(l.tipPath + ".tmp.*"); len(matches) != 0 {
		t.Fatalf("a failed rename must remove its tmp file: %v", matches)
	}
}

func TestAppendLineAndReplaceTip_AnUncreatableLedgerDirIsNamed(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(filepath.Join(blocker, "evolve"))

	err := l.appendLineAndReplaceTip(map[string]int{"entry_seq": 0}, 0)

	if err == nil || !strings.Contains(err.Error(), "ledger mkdir") {
		t.Fatalf("err = %v, want a ledger mkdir failure", err)
	}
}

func TestAppendLineAndReplaceTip_WritesTheSharedTipEncoding(t *testing.T) {
	t.Parallel()
	l := New(t.TempDir())

	if err := l.appendLineAndReplaceTip(map[string]int{"entry_seq": 0}, 0); err != nil {
		t.Fatal(err)
	}

	line, _ := os.ReadFile(l.ledgerPath)
	tip, err := os.ReadFile(l.tipPath)
	if want := ledgertip.Format(0, sha256Hex(bytes.TrimSuffix(line, []byte("\n")))); err != nil || string(tip) != want {
		t.Fatalf("tip = %q (err %v), want %q", tip, err, want)
	}
}
