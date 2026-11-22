package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestRunGCCheck_CLIModesAndErrors(t *testing.T) {
	t.Parallel()
	hub := gittest.Fixture(t)
	hub.Write("README.md", "hub\n")
	hub.Git("add", "-A")
	hub.Git("commit", "-q", "-m", "init")
	origin := gittest.Bare(t)
	hub.Git("remote", "add", "origin", origin.Dir)
	hub.Git("push", "-q", "origin", "main")
	wt := filepath.Join(hub.Dir, ".evolve", "worktrees", "cycle-cli-1")
	hub.Git("worktree", "add", "-q", "-b", "cycle-cli-1", wt, "main")
	if err := os.WriteFile(filepath.Join(wt, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"check", "--project-root", hub.Dir, "--json"}, nil, &stdout, &stderr)
	if rc != cliExitOK {
		t.Fatalf("json check rc=%d stderr=%s", rc, stderr.String())
	}
	var rep gc.CheckReport
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("decode report: %v\n%s", err, stdout.String())
	}
	if len(rep.Checks) != 2 || rep.Checks[0].Name != "worktrees" || len(rep.Checks[0].Items) != 1 || rep.Checks[1].Name != "temp" {
		t.Fatalf("unexpected report: %+v", rep)
	}

	stdout.Reset()
	stderr.Reset()
	rc = runGC([]string{"check", "--project-root", hub.Dir}, nil, &stdout, &stderr)
	if rc != cliExitOK || !strings.Contains(stdout.String(), "check worktrees: 1 item(s)") || !strings.Contains(stdout.String(), "FLAG-DIRTY") {
		t.Fatalf("text check rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGC([]string{"check", "--bogus"}, nil, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), "bogus") {
		t.Errorf("bad flag rc=%d stderr=%q", rc, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGC([]string{"check", "extra"}, nil, &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), "unexpected argument") {
		t.Errorf("extra arg rc=%d stderr=%q", rc, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	notRepo := t.TempDir()
	if rc := runGC([]string{"check", "--project-root", notRepo}, nil, &stdout, &stderr); rc != cliExitFailed || !strings.Contains(stderr.String(), "evolve gc check:") {
		t.Errorf("non-repo rc=%d stderr=%q", rc, stderr.String())
	}

	if _, ok := coreVerbOf("gc", []string{"check", "--json"}); ok {
		t.Errorf("coreVerbOf(gc check) must return false so json is not wrapped")
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGC([]string{"check", "--json"}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Errorf("cwd fallback rc=%d stderr=%q", rc, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGCCheckWith([]string{"--json"}, func() (string, error) { return "", errors.New("no cwd") }, filepath.Abs, &stdout, &stderr); rc != cliExitFailed {
		t.Errorf("failing getwd rc=%d, want %d", rc, cliExitFailed)
	}

	stdout.Reset()
	stderr.Reset()
	if rc := runGCCheckWith([]string{"--project-root", hub.Dir, "--json"}, os.Getwd, func(string) (string, error) { return "", errors.New("bad abs") }, &stdout, &stderr); rc != cliExitFailed {
		t.Errorf("failing abs rc=%d, want %d", rc, cliExitFailed)
	}
}
