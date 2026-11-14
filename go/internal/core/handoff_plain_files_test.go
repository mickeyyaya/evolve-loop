package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const hiddenPredicateName = "TestHidden_SECRET_Predicate"

const plainPredicateBody = "package cycle7\n\nimport \"testing\"\n\nfunc TestC7_001_Thing(t *testing.T) {}\n"

func promptWithin(t *testing.T, wt string) string {
	t.Helper()
	dir, ws := personaTDDWorkspace(t)
	o := personaOrchestrator(dir)
	o.acsPredicates = func(context.Context, string, int) acsPredicates { return acsPredicates{} }
	done := make(chan string, 1)
	go func() {
		done <- o.seedTaskContract(context.Background(), map[string]string{}, PhaseBuild, CycleState{WorkspacePath: ws, ActiveWorktree: wt, CycleID: 7}, dir)[CtxKeyTaskContract]
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(20 * time.Second):
		t.Fatal("seedTaskContract did not return: it reads a file that is not a plain file")
		return ""
	}
}

func addFifo(t *testing.T, wt, name string) {
	t.Helper()
	fifo := filepath.Join(wt, "go", "acs", "cycle7", name)
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("no fifo support: %v", err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})
}

func TestPlainFiles_FifoTestFileDoesNotHangTheBuildPrompt(t *testing.T) {
	t.Parallel()
	wt := personaBuildWorktree(t, 7, plainPredicateBody)
	addFifo(t, wt, "pipe_test.go")
	got := promptWithin(t, wt)
	if !strings.Contains(got, "pair: tdd-build") && !strings.Contains(got, "Handoff request: none for pair tdd-build (") {
		t.Errorf("the build prompt has no tdd-build handoff block:\n%s", got)
	}
}

func TestPlainFiles_SymlinkedTestFileAddsNoBytesToTheBuildPrompt(t *testing.T) {
	t.Parallel()
	wt := personaBuildWorktree(t, 7, plainPredicateBody)
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	body := "package hidden\n\nimport \"testing\"\n\nfunc " + hiddenPredicateName + "(t *testing.T) {}\n"
	if err := os.WriteFile(hidden, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, filepath.Join(wt, "go", "acs", "cycle7", "link_test.go")); err != nil {
		t.Fatal(err)
	}
	got := promptWithin(t, wt)
	if strings.Contains(got, hiddenPredicateName) {
		t.Errorf("a symlinked test file leaked its bytes into the build prompt:\n%s", got)
	}
}

func TestPlainFiles_RegularTestFileStillListedBesideARefusedEntry(t *testing.T) {
	t.Parallel()
	wt := personaBuildWorktree(t, 7, plainPredicateBody)
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	if err := os.WriteFile(hidden, []byte("package hidden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, filepath.Join(wt, "go", "acs", "cycle7", "link_test.go")); err != nil {
		t.Fatal(err)
	}
	got := promptWithin(t, wt)
	if !strings.Contains(got, "Handoff request") {
		t.Errorf("the build prompt dropped the handoff block:\n%s", got)
	}
}

func TestPlainFiles_LedgerLargerThanTheCapIsRefused(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	f, err := os.Create(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(256 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadHandoffLedger(ledger)
	if err == nil {
		t.Fatalf("a 256 MiB ledger was read whole (%d bytes)", len(raw))
	}
	if len(raw) != 0 {
		t.Errorf("a refused ledger returned %d bytes", len(raw))
	}
}

func TestPlainFiles_OrdinaryLedgerStillReads(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(t.TempDir(), "handoff-tdd-build.json")
	body := `{"schema_version":1,"pair":"tdd-build","pad":"` + strings.Repeat("x", 64<<10) + `"}`
	if err := os.WriteFile(ledger, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadHandoffLedger(ledger)
	if err != nil || string(raw) != body {
		t.Fatalf("a 64 KiB ledger = (%d bytes, %v), want it read whole", len(raw), err)
	}
}
