package ledger

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// ledger_crossproc_test.go — CA.1 (concurrency-factory plan, Track C-A):
// FileLedger.Append must be cross-PROCESS safe, not just cross-goroutine.
// The in-process sync.Mutex cannot serialize two `evolve` processes
// (fleet supervisor + a cycle, or two concurrent batches — the 278/279
// two-session launch race class): without an OS-level lock the
// tip-read→append→tip-write window interleaves and the hash chain breaks.

const stressDirEnv = "EVOLVE_LEDGER_STRESS_DIR"
const stressN = 150

// stressEntry builds a minimal distinguishable entry for the stress test.
func stressEntry(kind string, i int) core.LedgerEntry {
	return core.LedgerEntry{TS: fmt.Sprintf("2026-06-11T00:00:%02dZ", i%60), Cycle: 9000 + i, Role: "stress", Kind: kind}
}

// TestHelperLedgerAppender is not a real test: it is the child process body
// for the two-process stress test below. Gated on the env var so a normal
// `go test` run skips it instantly.
func TestHelperLedgerAppender(t *testing.T) {
	dir := os.Getenv(stressDirEnv)
	if dir == "" {
		t.Skip("helper process body; run via TestAppend_TwoProcessStress")
	}
	l := New(dir)
	for i := 0; i < stressN; i++ {
		if err := l.Append(context.Background(), stressEntry("stress-child", i)); err != nil {
			fmt.Fprintf(os.Stderr, "child append %d: %v\n", i, err)
			os.Exit(1)
		}
	}
}

// TestAppend_TwoProcessStress — the CA.1 acceptance: two OS processes
// append concurrently; afterwards the chain verifies and every entry made
// it in with a unique, gapless entry_seq.
func TestAppend_TwoProcessStress(t *testing.T) {
	if testing.Short() {
		t.Skip("two-process stress skipped in -short")
	}
	dir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperLedgerAppender$", "-test.v=false")
	cmd.Env = append(os.Environ(), stressDirEnv+"="+dir)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	l := New(dir)
	for i := 0; i < stressN; i++ {
		if err := l.Append(context.Background(), stressEntry("stress-parent", i)); err != nil {
			t.Fatalf("parent append %d: %v", i, err)
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("child process failed: %v", err)
	}

	if err := l.Verify(context.Background()); err != nil {
		t.Fatalf("chain broken after two-process append: %v", err)
	}
	it, err := l.Iter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	seqs := map[int]bool{}
	n := 0
	for {
		e, ok, ierr := it.Next()
		if ierr != nil {
			t.Fatal(ierr)
		}
		if !ok {
			break
		}
		if seqs[e.EntrySeq] {
			t.Errorf("duplicate entry_seq %d (lost-update interleave)", e.EntrySeq)
		}
		seqs[e.EntrySeq] = true
		n++
	}
	if n != 2*stressN {
		t.Errorf("entries = %d, want %d (appends lost)", n, 2*stressN)
	}
	for i := 0; i < 2*stressN; i++ {
		if !seqs[i] {
			t.Errorf("entry_seq %d missing (gap in chain)", i)
			break
		}
	}
}
