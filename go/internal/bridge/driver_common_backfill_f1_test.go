package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type scriptedTempF1 struct {
	*os.File
	writeErr error
	closeErr error
}

func (s scriptedTempF1) Write(b []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.File.Write(b)
}

func (s scriptedTempF1) Close() error {
	err := s.File.Close()
	if s.closeErr != nil {
		return s.closeErr
	}
	return err
}

func swapRelocateSeamsF1(t *testing.T, writeErr, closeErr, chmodErr error) {
	t.Helper()
	origCreate, origChmod := relocateCreateTemp, relocateChmod
	t.Cleanup(func() { relocateCreateTemp, relocateChmod = origCreate, origChmod })
	relocateCreateTemp = func(dir, pattern string) (relocateTempFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return scriptedTempF1{File: f, writeErr: writeErr, closeErr: closeErr}, nil
	}
	if chmodErr != nil {
		relocateChmod = func(string, os.FileMode) error { return chmodErr }
	}
}

func renameBlockedPairF1(t *testing.T) (src, dst string) {
	t.Helper()
	root := t.TempDir()
	src = filepath.Join(root, "src.md")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst = filepath.Join(root, "out", "dst.md")
	if err := os.MkdirAll(filepath.Join(dst, "occupied"), 0o755); err != nil {
		t.Fatal(err)
	}
	return src, dst
}

func assertNoTempLeftF1(t *testing.T, dst string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(dst))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file %s left behind after a failed relocation", e.Name())
		}
	}
}

func TestRelocateFileBackfillF1_CopyFallbackFailuresNameTheirStepAndLeaveNoTemp(t *testing.T) {
	boom := errors.New("injected")
	cases := []struct {
		name                  string
		write, closeE, chmodE error
		wantStep              string
		wantWraps             error
	}{
		{name: "write", write: boom, wantStep: "relocate: write dst tmp", wantWraps: boom},
		{name: "close", closeE: boom, wantStep: "relocate: close dst tmp", wantWraps: boom},
		{name: "chmod", chmodE: boom, wantStep: "relocate: chmod dst tmp", wantWraps: boom},
		{name: "rename tmp into place", wantStep: "relocate: rename tmp into place"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, dst := renameBlockedPairF1(t)
			swapRelocateSeamsF1(t, tc.write, tc.closeE, tc.chmodE)
			err := relocateFile(src, dst)
			if err == nil || !strings.HasPrefix(err.Error(), tc.wantStep) {
				t.Fatalf("err = %v, want prefix %q", err, tc.wantStep)
			}
			if tc.wantWraps != nil && !errors.Is(err, tc.wantWraps) {
				t.Fatalf("err %v does not wrap the injected failure", err)
			}
			if _, statErr := os.Stat(src); statErr != nil {
				t.Fatalf("a failed relocation must leave the source in place: %v", statErr)
			}
			assertNoTempLeftF1(t, dst)
		})
	}
}

func TestRelocateFileBackfillF1_ASourceThatCannotBeReadAfterTheRenameFailsIsReported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "srcdir")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "out", "dst.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := relocateFile(src, dst)
	if err == nil || !strings.HasPrefix(err.Error(), "relocate: read src") {
		t.Fatalf("err = %v, want the read-src failure", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "existing" {
		t.Fatalf("the destination must be untouched, got %q", got)
	}
}

func TestRelocateFileBackfillF1_SameDeviceMoveSkipsTheCopyFallback(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.md")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "out", "dst.md")
	copyFallbackRan := false
	origCreate := relocateCreateTemp
	t.Cleanup(func() { relocateCreateTemp = origCreate })
	relocateCreateTemp = func(dir, pattern string) (relocateTempFile, error) {
		copyFallbackRan = true
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return scriptedTempF1{File: f}, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := relocateFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if copyFallbackRan {
		t.Fatal("a same-device rename succeeds, so the copy fallback must not run")
	}
	if got, err := os.ReadFile(dst); err != nil || string(got) != "payload" {
		t.Fatalf("dst = %q, %v", got, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("the source must be gone after the move: %v", err)
	}
}
