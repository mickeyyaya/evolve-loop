//go:build integration

package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func requireLsof(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof not on PATH")
	}
}

func TestHolders_NamesTheProcessHoldingAFileUnderTheDir(t *testing.T) {
	requireLsof(t)
	dir := t.TempDir()
	held, err := os.Create(filepath.Join(dir, "held.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	got := holders(dir)

	want := "(PID " + strconv.Itoa(os.Getpid()) + ")"
	if !strings.HasPrefix(got, "holding processes: ") || !strings.Contains(got, want) || strings.Count(got, want) != 1 {
		t.Fatalf("holders = %q, want this process named exactly once as %s", got, want)
	}
}

func TestHolders_AnIdleDirHasNoHolders(t *testing.T) {
	requireLsof(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "closed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := holders(dir); got != "holding process: none found by lsof +D "+dir {
		t.Fatalf("holders = %q, want the none-found diagnostic naming the dir", got)
	}
}

func TestHolders_AnUnrunnableLsofIsADiagnosticNotAPanic(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "lsof"), []byte("not an executable format"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	got := holders(t.TempDir())

	if !strings.HasPrefix(got, "holding process: diagnostic unavailable (lsof: ") || !strings.Contains(got, "exec format error") {
		t.Fatalf("holders = %q, want the unavailable diagnostic with the launch error", got)
	}
}

func TestHolders_LsofExitStatusIsJudgedByItsOutput(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'COMMAND PID USER\\ngit 4242 me\\ngit 4242 me\\nsleep 77 me\\n'\nexit 1\n"
	fakeclitest.Install(t, filepath.Join(bin, "lsof"), script)
	t.Setenv("PATH", bin)

	if got := holders(t.TempDir()); got != "holding processes: git (PID 4242), sleep (PID 77)" {
		t.Fatalf("holders = %q, want both distinct holders despite lsof's exit 1", got)
	}
}

func TestHolders_NoLsofOnPathIsNamed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if got := holders(t.TempDir()); got != "holding process: diagnostic unavailable (lsof not on PATH)" {
		t.Fatalf("holders = %q, want the lsof-not-on-PATH diagnostic", got)
	}
}
