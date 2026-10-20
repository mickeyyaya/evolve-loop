package treefence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func shimGitFailingWriteTree(t *testing.T, times int) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	binDir := t.TempDir()
	count := filepath.Join(binDir, "count")
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = write-tree ]; then n=$(cat %[1]q 2>/dev/null || echo 0); n=$((n+1)); echo $n > %[1]q; if [ $n -le %[2]d ]; then echo \"shim: write-tree failure $n\" >&2; exit 3; fi; fi\nexec %[3]q \"$@\"\n", count, times, realGit)
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBegin_ATransientSnapshotFailureIsRetakenOnce(t *testing.T) {
	repo := initRepo(t)
	shimGitFailingWriteTree(t, 1)

	f := Begin(context.Background(), repo, true)
	if err := os.WriteFile(filepath.Join(repo, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := f.End(context.Background())

	if out.TakeErr != nil || !out.Verified || len(out.Restored) != 1 {
		t.Errorf("outcome = %+v, want the retaken snapshot to fence the phase: restored stray.txt and verified", out)
	}
	if f.RetakenAfter() == nil || !strings.Contains(f.RetakenAfter().Error(), "write-tree failure 1") || out.RetakenAfter == nil {
		t.Errorf("RetakenAfter = %v, outcome %v, want the first failure kept: the retry must not hide the failure the open item tracks", f.RetakenAfter(), out.RetakenAfter)
	}
	diags := out.Diagnostics("audit")
	if len(diags) == 0 || !strings.Contains(diags[0].Message, "snapshot retaken after:") || !strings.Contains(diags[0].Message, "write-tree failure 1") {
		t.Errorf("diagnostics = %+v, want a warning that names the first snapshot failure", diags)
	}
}

func TestBegin_ASecondSnapshotFailureNamesBothAttempts(t *testing.T) {
	repo := initRepo(t)
	shimGitFailingWriteTree(t, 2)

	err := Begin(context.Background(), repo, true).TakeErr()

	if err == nil || !strings.Contains(err.Error(), "write-tree failure 1") || !strings.Contains(err.Error(), "retry: ") || !strings.Contains(err.Error(), "write-tree failure 2") {
		t.Errorf("TakeErr = %v, want the first failure and the failure of the one retry", err)
	}
}

func TestBegin_ADispatchThatEndsDuringTheWaitDoesNotRetake(t *testing.T) {
	repo := initRepo(t)
	shimGitFailingWriteTree(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), retakeDelay/2)
	defer cancel()

	err := Begin(ctx, repo, true).TakeErr()

	if err == nil || !strings.HasSuffix(err.Error(), "; retry: "+context.DeadlineExceeded.Error()) {
		t.Errorf("TakeErr = %v, want the first failure and the context error of the wait, with no second snapshot", err)
	}
}

func TestFence_ANilFenceWasNeverRetaken(t *testing.T) {
	t.Parallel()
	var f *Fence
	if f.RetakenAfter() != nil || Begin(context.Background(), "", true).RetakenAfter() != nil {
		t.Error("a nil or inert fence reports a retake")
	}
}
