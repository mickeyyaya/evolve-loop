package tmuxtest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMain_ASocketThatCannotBeSelectedRunsNothingAndFails(t *testing.T) {
	prev := setenv
	t.Cleanup(func() { setenv = prev })
	setenv = func(string, string) error { return errors.New("env frozen") }
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code != 1 || ran {
		t.Fatalf("Main = %d, ran = %v; want 1 without running the tests", code, ran)
	}
}

func TestMain_ASocketDirThatCannotBeRegisteredStillRunsTheTests(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("TMUX_TMPDIR", file)
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code != 0 || !ran {
		t.Fatalf("Main = %d, ran = %v; a failed registration must not stop the run", code, ran)
	}
}
