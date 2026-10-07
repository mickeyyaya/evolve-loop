package ledger

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func peerAppendsUnlessTheChainIsLocked(t *testing.T, peer *FileLedger) bool {
	t.Helper()
	release, held, err := flock.TryLock(peer.lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if held {
		return false
	}
	release()
	if err := peer.Append(context.Background(), core.LedgerEntry{Role: "peer", Kind: "k"}); err != nil {
		t.Fatal(err)
	}
	return true
}

func TestVerify_APeerAppendBetweenTheChainAndTipReadsIsNotABreak(t *testing.T) {
	for name, verify := range map[string]func(*FileLedger) error{
		"Verify":     func(l *FileLedger) error { return l.Verify(context.Background()) },
		"VerifyDeep": func(l *FileLedger) error { return l.VerifyDeep(context.Background()) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			l := New(dir)
			if err := l.Append(context.Background(), core.LedgerEntry{Role: "builder", Kind: "k"}); err != nil {
				t.Fatal(err)
			}
			peer := New(dir)
			turns := map[string]int{}
			var err error
			withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
				if path == l.tipPath {
					turns[path]++
					peerAppendsUnlessTheChainIsLocked(t, peer)
				}
				body, rerr := os.ReadFile(path)
				if path == l.ledgerPath {
					turns[path]++
					peerAppendsUnlessTheChainIsLocked(t, peer)
				}
				return body, rerr
			}}, func() { err = verify(l) })

			if turns[l.ledgerPath] == 0 || turns[l.tipPath] == 0 {
				t.Fatalf("the peer had no turn after the chain read or before the tip read (turns=%v)", turns)
			}
			if err != nil {
				t.Fatalf("a peer's append between the chain read and the tip read is not a broken chain: %v", err)
			}
			if !peerAppendsUnlessTheChainIsLocked(t, peer) {
				t.Fatal("verification left the chain locked")
			}
			if err := verify(l); err != nil {
				t.Fatalf("the chain with the peer's append: %v", err)
			}
		})
	}
}

func TestVerify_ADirectoryWithNoLedgerVerifiesWithoutCreatingALock(t *testing.T) {
	for name, verify := range map[string]func(*FileLedger) error{
		"Verify":     func(l *FileLedger) error { return l.Verify(context.Background()) },
		"VerifyDeep": func(l *FileLedger) error { return l.VerifyDeep(context.Background()) },
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()

			err := verify(New(dir))

			if err != nil {
				t.Fatalf("a directory with no ledger has nothing to break: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "ledger.lock")); !os.IsNotExist(statErr) {
				t.Fatalf("verifying an absent ledger created %s (stat err=%v): a read must not leave state behind", "ledger.lock", statErr)
			}
		})
	}
}

func TestVerify_AnAbsentLedgerIsNotRead(t *testing.T) {
	var read []string
	withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
		read = append(read, path)
		return os.ReadFile(path)
	}}, func() {
		if err := New(t.TempDir()).Verify(context.Background()); err != nil {
			t.Fatalf("a directory with no ledger has nothing to break: %v", err)
		}
	})

	if len(read) != 0 {
		t.Fatalf("Verify read %v from a directory with no ledger: an absent ledger answers without an unlocked read", read)
	}
}

func sharedLockIsFree(t *testing.T, path string) error {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return err
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func TestVerify_HoldsTheChainLockSharedSoReadersDoNotSerialize(t *testing.T) {
	for name, verify := range map[string]func(*FileLedger) error{
		"Verify":     func(l *FileLedger) error { return l.Verify(context.Background()) },
		"VerifyDeep": func(l *FileLedger) error { return l.VerifyDeep(context.Background()) },
	} {
		t.Run(name, func(t *testing.T) {
			l := New(t.TempDir())
			if err := l.Append(context.Background(), core.LedgerEntry{Role: "builder", Kind: "k"}); err != nil {
				t.Fatal(err)
			}
			second := errors.New("the second reader never tried")
			withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
				if path == l.ledgerPath {
					second = sharedLockIsFree(t, l.lockPath)
				}
				return os.ReadFile(path)
			}}, func() {
				if err := verify(l); err != nil {
					t.Fatal(err)
				}
			})

			if second != nil {
				t.Fatalf("a second reader could not share the lock while the chain was read: %v", second)
			}
		})
	}
}

func TestVerify_ReadsALedgerInADirectoryItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only directory, so this run cannot show a read-only ledger")
	}
	dir := t.TempDir()
	l := New(dir)
	if err := l.Append(context.Background(), core.LedgerEntry{Role: "builder", Kind: "k"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ledger.jsonl", "ledger.tip", "ledger.lock"} {
		if err := os.Chmod(filepath.Join(dir, name), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	for name, verify := range map[string]func() error{
		"Verify":     func() error { return l.Verify(context.Background()) },
		"VerifyDeep": func() error { return l.VerifyDeep(context.Background()) },
	} {
		if err := verify(); err != nil {
			t.Errorf("%s on a read-only ledger = %v, want OK: a reader needs no write access", name, err)
		}
	}
}

func TestVerify_FailsLoudlyWhenTheChainLockCannotBeOpened(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a file of mode 000, so this run cannot show a lock that cannot be opened")
	}
	dir := t.TempDir()
	l := New(dir)
	if err := l.Append(context.Background(), core.LedgerEntry{Role: "builder", Kind: "k"}); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, "ledger.lock")
	if err := os.Chmod(lock, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lock, 0o644) })

	for name, verify := range map[string]func() error{
		"Verify":     func() error { return l.Verify(context.Background()) },
		"VerifyDeep": func() error { return l.VerifyDeep(context.Background()) },
	} {
		if err := verify(); err == nil || !strings.Contains(err.Error(), "ledger.lock") {
			t.Errorf("%s with a lock it cannot open = %v, want an error naming the lock: only an absent lock means no writer, any other failure must not read the chain unlocked", name, err)
		}
	}
}
