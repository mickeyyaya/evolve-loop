package cyclesimulator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendSimLedger_AnEncodeFailureWritesNeitherTheLineNorTheTip(t *testing.T) {
	orig := simLedgerCompact
	t.Cleanup(func() { simLedgerCompact = orig })
	boom := errors.New("encode failed")
	simLedgerCompact = func(map[string]any) (string, error) { return "", boom }
	root := t.TempDir()
	ledger := filepath.Join(root, ".evolve", "ledger.jsonl")
	artifact := filepath.Join(root, "a.md")
	if err := os.WriteFile(artifact, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := appendSimLedger(ledger, 1, "scout", artifact, "tok", root, func() time.Time { return time.Unix(0, 0) })

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	for _, name := range []string{"ledger.jsonl", "ledger.tip"} {
		if _, statErr := os.Stat(filepath.Join(root, ".evolve", name)); !os.IsNotExist(statErr) {
			t.Errorf("%s must not exist after an encode failure: %v", name, statErr)
		}
	}
}

func TestAppendLedgerLine_AWriteFailureClosesTheFileAndReturnsTheCause(t *testing.T) {
	orig := simLedgerWrite
	t.Cleanup(func() { simLedgerWrite = orig })
	boom := errors.New("disk full")
	var written string
	simLedgerWrite = func(f *os.File, s string) (int, error) { written = s; return 0, boom }
	path := filepath.Join(t.TempDir(), "sub", "ledger.jsonl")

	err := appendLedgerLine(path, `{"a":1}`)

	if !errors.Is(err, boom) || written != "{\"a\":1}\n" {
		t.Fatalf("err=%v written=%q", err, written)
	}
	if err := appendLedgerLine(filepath.Join(path, "child"), "x"); err == nil || strings.Contains(err.Error(), "disk full") {
		t.Errorf("a path below a file fails at directory creation, not at write: %v", err)
	}
}
