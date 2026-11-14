package request_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

const validPredicate = "package cycle9001\n\nimport \"testing\"\n\nfunc TestC9001_001_Only(t *testing.T) {}\n"

const hiddenPredicate = "package hidden\n\nimport \"testing\"\n\nfunc TestHidden_SECRET_Predicate(t *testing.T) {}\n"

type listing struct {
	req handoff.Request
	err error
}

func listWithin(t *testing.T, dir string) listing {
	t.Helper()
	done := make(chan listing, 1)
	go func() {
		req, err := request.TDDBuild(dir)
		done <- listing{req, err}
	}()
	select {
	case got := <-done:
		return got
	case <-time.After(20 * time.Second):
		t.Fatal("TDDBuild did not return: it reads a file that is not a plain file")
		return listing{}
	}
}

func askIDs(req handoff.Request) []string {
	var ids []string
	for _, ask := range req.Asks {
		ids = append(ids, ask.ID)
	}
	return ids
}

func TestTDDBuild_FifoTestFileDoesNotHang(t *testing.T) {
	dir := writePackage(t, map[string]string{"a_test.go": validPredicate})
	fifo := filepath.Join(dir, "pipe_test.go")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("no fifo support: %v", err)
	}
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_RDWR, 0); err == nil {
			_ = f.Close()
		}
	})
	got := listWithin(t, dir)
	if got.err == nil && len(got.req.Asks) != 1 {
		t.Errorf("asks = %v, want only the regular file's predicate", askIDs(got.req))
	}
}

func TestTDDBuild_SymlinkedTestFileContributesNothing(t *testing.T) {
	dir := writePackage(t, map[string]string{"a_test.go": validPredicate})
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	if err := os.WriteFile(hidden, []byte(hiddenPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, filepath.Join(dir, "link_test.go")); err != nil {
		t.Fatal(err)
	}
	got := listWithin(t, dir)
	for _, id := range askIDs(got.req) {
		if id == "TestHidden_SECRET_Predicate" {
			t.Errorf("a symlinked test file contributed %s", id)
		}
	}
}

func TestTDDBuild_SymlinkOnlyPackageHasNoPredicates(t *testing.T) {
	dir := writePackage(t, nil)
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	if err := os.WriteFile(hidden, []byte(hiddenPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hidden, filepath.Join(dir, "link_test.go")); err != nil {
		t.Fatal(err)
	}
	got := listWithin(t, dir)
	if got.err == nil {
		t.Errorf("a package whose only test file is a symlink listed %v", askIDs(got.req))
	}
}

func TestTDDBuild_HardLinkedTestFileContributesNothing(t *testing.T) {
	dir := writePackage(t, map[string]string{"a_test.go": validPredicate})
	hidden := filepath.Join(t.TempDir(), "hidden.go")
	if err := os.WriteFile(hidden, []byte(hiddenPredicate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(hidden, filepath.Join(dir, "hard_test.go")); err != nil {
		t.Skipf("no hard link support: %v", err)
	}
	got := listWithin(t, dir)
	for _, id := range askIDs(got.req) {
		if id == "TestHidden_SECRET_Predicate" {
			t.Errorf("a hard-linked test file contributed %s", id)
		}
	}
}
