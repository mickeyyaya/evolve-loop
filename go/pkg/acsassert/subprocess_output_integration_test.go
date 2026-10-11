//go:build integration

package acsassert

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSubprocessOutput_AnUnlaunchableBinaryIsANonExitError(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "broken-tool"), []byte("not an executable format"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	stdout, stderr, code, err := SubprocessOutput("broken-tool", "arg")

	if code != -1 || stdout != "" || stderr != "" || !errors.Is(err, syscall.ENOEXEC) || errors.Is(err, ErrSubprocessNotFound) {
		t.Fatalf("SubprocessOutput = %q, %q, %d, %v; want -1 and the wrapped ENOEXEC launch error", stdout, stderr, code, err)
	}
	if !strings.HasPrefix(err.Error(), "subprocess broken-tool: ") {
		t.Fatalf("err = %v, want it to name the subprocess", err)
	}
}

func TestSubprocessOutput_SeparatesStreamsAndReportsTheExitCode(t *testing.T) {
	for _, tc := range []struct {
		script, stdout, stderr string
		code                   int
		wantErr                string
	}{
		{"printf out; printf err >&2", "out", "err", 0, ""},
		{"printf partial; printf boom >&2; exit 9", "partial", "boom", 9, "subprocess sh exited 9"},
	} {
		stdout, stderr, code, err := SubprocessOutput("sh", "-c", tc.script)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if stdout != tc.stdout || stderr != tc.stderr || code != tc.code || gotErr != tc.wantErr {
			t.Fatalf("SubprocessOutput(%q) = %q, %q, %d, %q; want %q, %q, %d, %q", tc.script, stdout, stderr, code, gotErr, tc.stdout, tc.stderr, tc.code, tc.wantErr)
		}
	}
}
