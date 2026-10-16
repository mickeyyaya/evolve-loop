//go:build integration

package verifyeval

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDefaultRunner_Success(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("/bin/true not present")
	}
	stdout, stderr, exit, err := DefaultRunner(context.Background(), "", "/bin/true")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if exit != 0 {
		t.Errorf("exit=%d, want 0", exit)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("expected empty stdio; stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestDefaultRunner_NonZeroExit(t *testing.T) {
	if _, err := os.Stat("/bin/false"); err != nil {
		t.Skip("/bin/false not present")
	}
	_, _, exit, err := DefaultRunner(context.Background(), "", "/bin/false")
	if err != nil {
		t.Errorf("err=%v, want nil (exit-error mapped to exitCode)", err)
	}
	if exit != 1 {
		t.Errorf("exit=%d, want 1", exit)
	}
}

func TestDefaultRunner_StdoutCaptured(t *testing.T) {
	stdout, _, _, err := DefaultRunner(context.Background(), "", "echo hello-from-runner")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "hello-from-runner") {
		t.Errorf("stdout missing echo: %q", stdout)
	}
}

func TestDefaultRunner_BinaryMissing(t *testing.T) {
	_, _, _, err := DefaultRunner(context.Background(), "", "/no/such/binary/zzz")
	_ = err
}
