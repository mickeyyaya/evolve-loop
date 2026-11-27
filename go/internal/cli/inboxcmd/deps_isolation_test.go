package inboxcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restoreHostAfter(t *testing.T) *Deps {
	t.Helper()
	before := current.Load()
	t.Cleanup(func() { current.Store(before) })
	return before
}

func TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint(t *testing.T) {
	restoreHostAfter(t)
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
				t.Errorf("runInboxLint panicked on the host a Deps{} Run left behind: %v", r)
			}
		}()
		var lintOut, lintErr bytes.Buffer
		if rc := runInboxLint(fixtureRoot, nil, &lintOut, &lintErr); rc != exitOK {
			t.Errorf("runInboxLint rc = %d, want %d (stdout %q, stderr %q)", rc, exitOK, lintOut.String(), lintErr.String())
		}
	})
}

func TestRun_RestoresThePreviousHostOnReturn(t *testing.T) {
	before := restoreHostAfter(t)
	var out, errb bytes.Buffer
	if rc := Run([]string{"--help"}, strings.NewReader(""), &out, &errb, Deps{}); rc != exitOK {
		t.Fatalf("Run --help rc = %d, stderr %q", rc, errb.String())
	}
	if after := current.Load(); after != before {
		t.Errorf("Run left the host at %p, want the previous host %p restored", after, before)
	}
}

func TestRunMover_RestoresThePreviousHostOnReturn(t *testing.T) {
	before := restoreHostAfter(t)
	var out, errb bytes.Buffer
	if rc := RunMover([]string{"--help"}, strings.NewReader(""), &out, &errb, testDeps()); rc != exitOK {
		t.Fatalf("RunMover --help rc = %d, stderr %q", rc, errb.String())
	}
	if after := current.Load(); after != before {
		t.Errorf("RunMover left the host at %p, want the previous host %p restored", after, before)
	}
}
