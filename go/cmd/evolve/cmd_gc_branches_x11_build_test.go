package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestX11_RemoteSweepPlanSkipAndBundleFailurePaths(t *testing.T) {
	t.Parallel()
	wSkip := &remoteGCWorld{failGit: map[string]bool{"ls-remote": true}}
	rSkip, _, errSkip := remoteGCRun(wSkip, false)
	if failed := rSkip.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); failed {
		t.Fatalf("plan error should warn and skip, got failed=true: %s", errSkip.String())
	}
	if !strings.Contains(errSkip.String(), "remote branch sweep skipped") {
		t.Errorf("stderr = %q, want skipped warning", errSkip.String())
	}

	wBundle := &remoteGCWorld{failGit: map[string]bool{"bundle": true}}
	rBundle, _, errBundle := remoteGCRun(wBundle, false)
	if failed := rBundle.remoteSweep(t.TempDir(), t.TempDir(), enforcePolicyGC()); !failed {
		t.Fatalf("bundle error should fail run")
	}
	if !strings.Contains(errBundle.String(), "remote branch sweep partial") {
		t.Errorf("stderr = %q, want partial error", errBundle.String())
	}

	var out, errOut bytes.Buffer
	rCheck := newGCRun(context.Background(), false, &out, &errOut)
	rCheck.exec = sysexec.DefaultRunner
	if rc := runGC([]string{"check", "extra"}, nil, &out, &errOut); rc != cliExitUsage {
		t.Errorf("check extra arg rc = %d, want %d", rc, cliExitUsage)
	}
	if rc := runGC([]string{"check", "--bogus"}, nil, &out, &errOut); rc != cliExitUsage {
		t.Errorf("check bogus flag rc = %d, want %d", rc, cliExitUsage)
	}
	if rc := runGC([]string{"check", "--project-root", t.TempDir()}, nil, &out, &errOut); rc != cliExitFailed {
		t.Errorf("check non-repo rc = %d, want %d", rc, cliExitFailed)
	}
	bp := gc.BranchesPolicy{OwnedPrefixes: []string{"custom/"}}
	if bp.IsOwned("cycle-1") || !bp.IsOwned("custom/a") || bp.IsOwned("stray/x") {
		t.Errorf("BranchesPolicy.IsOwned failed")
	}
	_ = gc.Policy{}
}
