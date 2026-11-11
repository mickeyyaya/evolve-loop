package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeGHFailingRunList(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
api) printf '{"sha":"` + ciWatchEventSHA + `"}' ;;
*) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunCIWatch_AWorkingDirectoryFaultExitsIOWhenNoRootIsGiven(t *testing.T) {
	breakWorkingDirectory(t)
	var stdout, stderr bytes.Buffer

	code := runCIWatch([]string{"--sha", ciWatchEventSHA}, &stdout, &stderr)

	if code != exitIO {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitIO, stderr.String())
	}
	if !strings.Contains(stderr.String(), ciWatchPrefix) || !strings.Contains(stderr.String(), "getwd") {
		t.Errorf("stderr %q must carry the verb prefix and the working directory fault", stderr.String())
	}
}

func TestRunCIWatch_AFailedRunLookupExitsIOAndNamesTheWorkflow(t *testing.T) {
	useRealRootEvents(t)
	fakeGHFailingRunList(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := runCIWatch([]string{"--sha", ciWatchEventSHA, "--workflow", "required.yml", "--project-root", root}, &stdout, &stderr)

	if code != exitIO {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitIO, stderr.String())
	}
	if !strings.Contains(stderr.String(), ciWatchPrefix+"required.yml: ") {
		t.Errorf("stderr %q must name the workflow whose lookup failed", stderr.String())
	}
	if got := channelledEvents(t, filepath.Join(root, ".evolve"), "signals", "ci.completed"); len(got) != 0 {
		t.Errorf("ci.completed events = %+v, want none: no run completed", got)
	}
}
