//go:build unix

package opscmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWriteConsoleLease_AFailedWriteIsReportedAndLeavesNoLeaseOrTempFileF2(t *testing.T) {
	dir := t.TempDir()
	lease := filepath.Join(dir, "lease.json")
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Skipf("no RLIMIT_FSIZE: %v", err)
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 0, Max: old.Max}); err != nil {
		t.Skipf("cannot lower RLIMIT_FSIZE: %v", err)
	}
	restore := func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Errorf("restore RLIMIT_FSIZE: %v", err)
		}
	}
	var out, errb bytes.Buffer

	rc := writeConsoleLease(lease, []string{"a.md"}, "r", time.Minute, &out, &errb)
	restore()

	entries, readErr := os.ReadDir(dir)
	if rc != 1 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "evolve console-lease: ") || readErr != nil || len(entries) != 0 {
		t.Errorf("rc=%d out=%q err=%q dir=%v (%v), want exit 1 and an empty dir", rc, out.String(), errb.String(), entries, readErr)
	}
}
