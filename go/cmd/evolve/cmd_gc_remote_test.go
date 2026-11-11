package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const remoteTipGC = "0123456789abcdef0123456789abcdef01234567"

type remoteGCWorld struct {
	calls   []string
	failGit map[string]bool
}

func (w *remoteGCWorld) run(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	line := name + " " + strings.Join(args, " ")
	w.calls = append(w.calls, line)
	switch {
	case name == "git" && args[0] == "ls-remote":
		_, _ = io.WriteString(stdout, remoteTipGC+"\trefs/heads/feat-x\n")
	case name == "git" && args[0] == "rev-parse":
		_, _ = io.WriteString(stdout, remoteTipGC)
	case name == "git" && args[0] == "bundle" && args[1] == "list-heads":
		_, _ = io.WriteString(stdout, remoteTipGC+" refs/remotes/origin/feat-x\n")
	case name == "gh" && args[0] == "pr":
		_, _ = io.WriteString(stdout, `[{"headRefName":"feat-x","headRefOid":"`+remoteTipGC+`","state":"MERGED"}]`)
	case name == "gh" && args[1] == "-X":
	case name == "gh":
		_, _ = io.WriteString(stdout, "[]")
	}
	if w.failGit[args[0]] {
		return 1, nil
	}
	return 0, nil
}

func (w *remoteGCWorld) ran(fragment string) bool {
	for _, c := range w.calls {
		if strings.Contains(c, fragment) {
			return true
		}
	}
	return false
}

func remoteGCRun(w *remoteGCWorld, dryRun bool) (gcRun, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	r := newGCRun(context.Background(), dryRun, &out, &errOut)
	r.exec = sysexec.RunFunc(w.run)
	return r, &out, &errOut
}

func enforcePolicyGC() gc.Policy {
	return gc.Policy{RemoteSweep: gcpolicy.RemoteSweepPolicy{Stage: "enforce"}}
}

func TestGCRemoteSweep_ADryRunReportsAtShadowEvenWhenPolicyEnforcesAndCallsNoDelete(t *testing.T) {
	t.Parallel()
	w := &remoteGCWorld{}
	r, out, errOut := remoteGCRun(w, true)
	if failed := r.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); failed {
		t.Fatalf("a dry run failed: %s", errOut)
	}
	if w.ran("-X DELETE") || w.ran("bundle create") {
		t.Errorf("a dry run mutated: %v", w.calls)
	}
	if !strings.Contains(out.String(), "remote WOULD-DELETE origin/feat-x") || !strings.Contains(out.String(), "1 remote branch(es) would be deleted at stage shadow (0 kept)") {
		t.Errorf("stdout = %q", out)
	}
}

func TestGCRemoteSweep_EnforceAppliesTheDeleteAfterTheBundle(t *testing.T) {
	t.Parallel()
	w := &remoteGCWorld{}
	r, out, errOut := remoteGCRun(w, false)
	if failed := r.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); failed {
		t.Fatalf("enforce failed: %s", errOut)
	}
	if !w.ran("bundle verify") || !w.ran("-X DELETE repos/{owner}/{repo}/git/refs/heads/feat-x") {
		t.Errorf("calls = %v, want a bundle verify and the API delete", w.calls)
	}
	if !strings.Contains(out.String(), "remote branch sweep — 1 branch(es) to delete at stage enforce (0 kept)") {
		t.Errorf("stdout = %q", out)
	}
}

func TestGCRemoteSweep_AnUnsetPolicyStaysAtShadowAndDeletesNothing(t *testing.T) {
	t.Parallel()
	w := &remoteGCWorld{}
	r, out, _ := remoteGCRun(w, false)
	if r.remoteSweep(t.TempDir(), t.TempDir(), gc.Policy{}) || w.ran("-X DELETE") {
		t.Fatalf("shadow failed or deleted: %v", w.calls)
	}
	if !strings.Contains(out.String(), "WOULD-DELETE") {
		t.Errorf("stdout = %q", out)
	}
}

func TestGCRemoteSweep_APlanFailureWarnsAndDoesNotFailTheRun(t *testing.T) {
	t.Parallel()
	w := &remoteGCWorld{failGit: map[string]bool{"ls-remote": true}}
	r, _, errOut := remoteGCRun(w, false)
	if failed := r.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); failed {
		t.Error("a plan failure failed the run")
	}
	if !strings.Contains(errOut.String(), "remote branch sweep skipped") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestGCRemoteSweep_AFailedBundleFailsTheRunAndKeepsTheBranch(t *testing.T) {
	t.Parallel()
	w := &remoteGCWorld{failGit: map[string]bool{"bundle": true}}
	r, _, errOut := remoteGCRun(w, false)
	if failed := r.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); !failed {
		t.Error("a failed bundle did not fail the run")
	}
	if w.ran("-X DELETE") || !strings.Contains(errOut.String(), "remote branch sweep partial") {
		t.Errorf("calls = %v, stderr = %q", w.calls, errOut)
	}
}

func TestGCRemoteSweep_AFailedRemoteDeleteFailsTheProjectRun(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	evolveDir := filepath.Join(repo.Dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gc":{"remote_sweep":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &remoteGCWorld{failGit: map[string]bool{"bundle": true}}
	r, _, errOut := remoteGCRun(w, false)
	r.exec = func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && args[0] != "ls-remote" && args[0] != "rev-parse" && args[0] != "bundle" && args[0] != "fetch" {
			return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
		}
		return w.run(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
	if rc := r.project(repo.Dir); rc != 1 {
		t.Errorf("project rc = %d, want 1 after a failed remote sweep; stderr=%q", rc, errOut)
	}
	if !strings.Contains(errOut.String(), "remote branch sweep partial") {
		t.Errorf("stderr = %q, want the partial remote sweep report", errOut)
	}
}

func TestGCNewRunKillSignalsOnlyTheProcessItIsGiven(t *testing.T) {
	t.Parallel()
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	r := newGCRun(context.Background(), false, io.Discard, io.Discard)
	if err := r.kill(child.Process.Pid); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if err := child.Wait(); err == nil {
		t.Error("the child exited cleanly; the kill closure did not signal it")
	}
}
