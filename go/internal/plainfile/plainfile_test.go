package plainfile_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plainfile"
)

const readLimit = 1 << 10

type readResult struct {
	body []byte
	err  error
}

func writeFile(t *testing.T, dir, name string, body []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readWithin(t *testing.T, path string) readResult {
	t.Helper()
	done := make(chan readResult, 1)
	go func() {
		body, err := plainfile.Read(path, readLimit)
		done <- readResult{body, err}
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(20 * time.Second):
		t.Fatal("Read did not return: it blocked on a file that is not a plain file")
		return readResult{}
	}
}

func makeFifo(t *testing.T, dir string) string {
	t.Helper()
	fifo := filepath.Join(dir, "pipe.go")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("no fifo support: %v", err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})
	return fifo
}

func TestRead_AnOrdinaryFileIsReturnedWhole(t *testing.T) {
	path := writeFile(t, t.TempDir(), "plain.go", []byte("package plain\n"))
	got := readWithin(t, path)
	if got.err != nil || string(got.body) != "package plain\n" {
		t.Fatalf("Read = (%q, %v), want the file bytes", got.body, got.err)
	}
}

func TestRead_AMissingFileReportsNotExist(t *testing.T) {
	got := readWithin(t, filepath.Join(t.TempDir(), "absent.go"))
	if !errors.Is(got.err, os.ErrNotExist) {
		t.Errorf("a missing file = %v, want os.ErrNotExist", got.err)
	}
}

func TestRead_AFifoIsRefusedWithoutBlocking(t *testing.T) {
	got := readWithin(t, makeFifo(t, t.TempDir()))
	if !errors.Is(got.err, plainfile.ErrNotPlain) {
		t.Errorf("a fifo = %v, want ErrNotPlain", got.err)
	}
}

func TestRead_ASymlinkIsRefusedAndItsTargetIsNotRead(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, t.TempDir(), "target.go", []byte("package secret\n"))
	link := filepath.Join(dir, "link.go")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got := readWithin(t, link)
	if !errors.Is(got.err, plainfile.ErrNotPlain) || len(got.body) != 0 {
		t.Errorf("a symlink = (%q, %v), want no bytes and ErrNotPlain", got.body, got.err)
	}
}

func TestRead_AHardLinkedFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	first := writeFile(t, dir, "first.go", []byte("package first\n"))
	second := filepath.Join(dir, "second.go")
	if err := os.Link(first, second); err != nil {
		t.Skipf("no hard link support: %v", err)
	}
	got := readWithin(t, second)
	if !errors.Is(got.err, plainfile.ErrNotPlain) || len(got.body) != 0 {
		t.Errorf("a hard link = (%q, %v), want no bytes and ErrNotPlain", got.body, got.err)
	}
}

func TestRead_AFileOverTheLimitIsRefusedAndOneAtTheLimitIsRead(t *testing.T) {
	dir := t.TempDir()
	atLimit := writeFile(t, dir, "at.go", bytes.Repeat([]byte("a"), readLimit))
	if got := readWithin(t, atLimit); got.err != nil || len(got.body) != readLimit {
		t.Errorf("a file at the limit = (%d bytes, %v), want all bytes", len(got.body), got.err)
	}
	over := writeFile(t, dir, "over.go", bytes.Repeat([]byte("a"), readLimit+1))
	if got := readWithin(t, over); !errors.Is(got.err, plainfile.ErrTooLarge) || len(got.body) != 0 {
		t.Errorf("a file over the limit = (%d bytes, %v), want no bytes and ErrTooLarge", len(got.body), got.err)
	}
}

func TestReadWith_AFileSwappedAfterTheCheckIsRefused(t *testing.T) {
	dir := t.TempDir()
	checked := writeFile(t, dir, "checked.go", []byte("package checked\n"))
	swappedIn := writeFile(t, dir, "swapped.go", []byte("package swapped\n"))
	swap := func(string) (*os.File, error) { return os.Open(swappedIn) }
	body, err := plainfile.ReadWith(checked, readLimit, swap)
	if !errors.Is(err, plainfile.ErrNotPlain) || len(body) != 0 {
		t.Errorf("a swapped file = (%q, %v), want no bytes and ErrNotPlain", body, err)
	}
}

func TestReadWith_AHandleThatCannotBeStatedIsRefused(t *testing.T) {
	path := writeFile(t, t.TempDir(), "plain.go", []byte("package plain\n"))
	closed := func(p string) (*os.File, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		_ = f.Close()
		return f, nil
	}
	body, err := plainfile.ReadWith(path, readLimit, closed)
	if !errors.Is(err, plainfile.ErrNotPlain) || len(body) != 0 {
		t.Errorf("an unstatable handle = (%q, %v), want no bytes and ErrNotPlain", body, err)
	}
}

func TestReadWith_AnOpenFailureIsReturnedAsIs(t *testing.T) {
	path := writeFile(t, t.TempDir(), "plain.go", []byte("package plain\n"))
	openErr := errors.New("open refused")
	failing := func(string) (*os.File, error) { return nil, openErr }
	if _, err := plainfile.ReadWith(path, readLimit, failing); !errors.Is(err, openErr) {
		t.Errorf("ReadWith = %v, want the open error", err)
	}
}

func TestOpen_AFifoOpensWithoutBlocking(t *testing.T) {
	fifo := makeFifo(t, t.TempDir())
	opened := make(chan error, 1)
	go func() {
		f, err := plainfile.Open(fifo)
		if f != nil {
			_ = f.Close()
		}
		opened <- err
	}()
	select {
	case err := <-opened:
		if err != nil {
			t.Errorf("Open on a fifo = %v, want a handle", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Open blocked on a fifo: the handle is not non-blocking")
	}
}

func TestOpen_ASymlinkIsNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target.go", []byte("package secret\n"))
	link := filepath.Join(dir, "link.go")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	f, err := plainfile.Open(link)
	if err == nil {
		_ = f.Close()
		t.Error("Open followed a symlink")
	}
}
