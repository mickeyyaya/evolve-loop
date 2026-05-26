package bridge

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"
)

// coverage_batch6_test.go — driver-level error returns reached by calling
// Launch directly (LaunchArgs pre-validates the prompt, so these paths
// are unreachable through it) + the small FS-helper branches.

func covDeps() Deps {
	return Deps{
		Stderr:            io.Discard,
		Stdout:            io.Discard,
		LookupEnv:         mapLookup(nil), // no ambient creds → guards don't fire
		NewChallengeToken: func() (string, error) { return "tok", nil },
		Now:               time.Now,
		Tmux:              &fakeTmux{},
		Sleep:             func(time.Duration) {},
	}
}

func TestHeadlessDrivers_PreparePromptError(t *testing.T) {
	ws := t.TempDir()
	cfg := &Config{
		Model:      "haiku",
		PromptFile: "/no/such/prompt-xyz.txt", // unreadable → preparePrompt errors
		Workspace:  ws,
		Artifact:   filepath.Join(ws, "a.md"),
		StdoutLog:  filepath.Join(ws, "o.log"),
		StderrLog:  filepath.Join(ws, "e.log"),
	}
	for _, d := range []Driver{claudePDriver{}, codexDriver{}, agyDriver{}} {
		code, err := d.Launch(context.Background(), cfg, covDeps())
		if code != ExitBadFlags || err == nil {
			t.Fatalf("%s preparePrompt error → (%d,%v), want (ExitBadFlags, err)", d.Name(), code, err)
		}
	}
}

func TestRunTmuxREPL_PreparePromptError(t *testing.T) {
	// Covers the shared runTmuxREPL preparePrompt-error path (all 3 tmux drivers).
	ws := t.TempDir()
	cfg := &Config{
		Model: "haiku", AllowBypass: true,
		PromptFile: "/no/such/prompt-xyz.txt",
		Workspace:  ws, Artifact: filepath.Join(ws, "a.md"),
		StdoutLog: filepath.Join(ws, "o.log"), StderrLog: filepath.Join(ws, "e.log"),
	}
	code, err := claudeTmuxDriver{}.Launch(context.Background(), cfg, covDeps())
	if code != ExitBadFlags || err == nil {
		t.Fatalf("claude-tmux preparePrompt error → (%d,%v), want (ExitBadFlags, err)", code, err)
	}
}

func TestEnsureDirs_EmptyFieldsSkipped(t *testing.T) {
	// Workspace "" → the d=="" continue branch; Dir("")=="." is creatable.
	if err := ensureDirs(&Config{}); err != nil {
		t.Fatalf("ensureDirs(empty) err: %v", err)
	}
}

func TestOpenDriverLogs_StderrCreateError(t *testing.T) {
	ws := t.TempDir()
	cfg := &Config{
		Workspace: ws,
		StdoutLog: filepath.Join(ws, "out.log"), // creatable
		StderrLog: ws,                           // a directory → Create fails
		Artifact:  filepath.Join(ws, "a.md"),
	}
	if _, _, _, err := openDriverLogs(cfg); err == nil {
		t.Fatal("openDriverLogs should fail when the stderr-log path is a directory")
	}
}
