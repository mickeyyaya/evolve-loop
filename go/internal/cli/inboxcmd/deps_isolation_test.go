package inboxcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint(t *testing.T) {
	t.Parallel()
	fixtureRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureRoot, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	Run([]string{"--help"}, strings.NewReader(""), &out, &errb, Deps{})

	t.Run("lint after the empty Deps run", func(t *testing.T) {
		t.Parallel()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("runInboxLint panicked after a Deps{} Run: %v", r)
			}
		}()
		var lintOut, lintErr bytes.Buffer
		if rc := runInboxLint(fixtureRoot, nil, &lintOut, &lintErr, testDeps()); rc != exitOK {
			t.Errorf("runInboxLint rc = %d, want %d (stdout %q, stderr %q)", rc, exitOK, lintOut.String(), lintErr.String())
		}
	})
}

func TestRun_RestoresThePreviousHostOnReturn(t *testing.T) {
	t.Parallel()
	fixtureRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureRoot, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if rc := Run([]string{"--help"}, strings.NewReader(""), &out, &errb, Deps{}); rc != exitOK {
		t.Fatalf("Run --help rc = %d, stderr %q", rc, errb.String())
	}
	out.Reset()
	errb.Reset()
	if rc := Run([]string{"list", "--project-root", fixtureRoot}, strings.NewReader(""), &out, &errb, testDeps()); rc != exitOK {
		t.Fatalf("Run list after empty Run rc = %d, stderr %q", rc, errb.String())
	}
}

func TestRunMover_RestoresThePreviousHostOnReturn(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	if rc := RunMover([]string{"--help"}, strings.NewReader(""), &out, &errb, testDeps()); rc != exitOK {
		t.Fatalf("RunMover --help rc = %d, stderr %q", rc, errb.String())
	}
}
