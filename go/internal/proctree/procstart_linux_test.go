//go:build linux

package proctree

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const statLine = "4242 (a) (b c) S 1 4242 4242 0 -1 4194560 100 0 0 0 1 2 0 0 20 0 1 0 98765 1000 10\n"

func procRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestStartOfAt_IsTheBootIDAndTheStartTicks(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"4242/stat": statLine, "sys/kernel/random/boot_id": "b0-1\n"})

	got, err := startOfAt(root, 4242)

	if err != nil || got != "b0-1:98765" {
		t.Errorf("startOfAt = %q, %v, want %q: field 22 after a comm with spaces and parens", got, err, "b0-1:98765")
	}
}

func TestStartOfAt_AGonePidIsESRCH(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"sys/kernel/random/boot_id": "b0-1\n"})

	got, err := startOfAt(root, 4242)

	if !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("startOfAt = %q, %v, want ESRCH", got, err)
	}
}

func TestStartOfAt_AnUnreadableStatIsReturned(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"4242/stat/x": ""})

	got, err := startOfAt(root, 4242)

	if err == nil || errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("startOfAt = %q, %v, want a read error that is not ESRCH", got, err)
	}
}

func TestStartOfAt_AShortStatIsMalformed(t *testing.T) {
	t.Parallel()
	for _, stat := range []string{"4242 (a) S 1 2 3\n", "4242 a S\n", "4242 (a) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18\n"} {
		root := procRoot(t, map[string]string{"4242/stat": stat, "sys/kernel/random/boot_id": "b0-1\n"})

		got, err := startOfAt(root, 4242)

		if !errors.Is(err, errProcStatMalformed) || got != "" {
			t.Errorf("startOfAt(%q) = %q, %v, want %v", stat, got, err, errProcStatMalformed)
		}
	}
}

func TestStartOfAt_AMissingBootIDIsAnError(t *testing.T) {
	t.Parallel()
	root := procRoot(t, map[string]string{"4242/stat": statLine})

	got, err := startOfAt(root, 4242)

	if !errors.Is(err, os.ErrNotExist) || got != "" {
		t.Errorf("startOfAt = %q, %v, want the boot id read error", got, err)
	}
}

func TestStartOf_ThisProcessReadsOneStableValue(t *testing.T) {
	t.Parallel()
	first, err := StartOf(os.Getpid())
	second, err2 := StartOf(os.Getpid())

	if err != nil || err2 != nil || first == "" || second != first {
		t.Errorf("StartOf(self) = %q, %v then %q, %v, want one stable value", first, err, second, err2)
	}
}

func TestStartOf_AnUnusedPidIsESRCH(t *testing.T) {
	t.Parallel()
	got, err := StartOf(math.MaxInt32)

	if !errors.Is(err, syscall.ESRCH) || got != "" {
		t.Errorf("StartOf(MaxInt32) = %q, %v, want ESRCH", got, err)
	}
}
