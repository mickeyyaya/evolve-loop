package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestScopedCoverFunc_ALostSingleFlightIsAnAbnormalEventNamingTheLockPath(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "go")
	writeCoverFixture(t, moduleDir)
	hold, err := verifylock.Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	saved := coverLockWait
	coverLockWait = 200 * time.Millisecond
	defer func() { coverLockWait = saved }()
	in := ReviewInput{Cycle: 7, Workspace: t.TempDir(), Stderr: &injectedStderr{}}
	lock, _ := verifylock.LockPath(root)

	_, _, status := scopedCoverFunc(context.Background(), in, moduleDir, []string{"./p"})

	raw, _ := os.ReadFile(filepath.Join(in.Workspace, "abnormal-events.jsonl"))
	if status != coverStatusOK || !strings.Contains(string(raw), `"verify-single-flight-unavailable"`) || !strings.Contains(string(raw), lock) || !strings.Contains(string(raw), "wait bound") {
		t.Errorf("status = %d, abnormal events = %q; want the unserialized run recorded with the lock path %s and the error", status, raw, lock)
	}
}

func TestRealGoUnitTest_TheFloorsUnitRunsDoNotTakeTheHostLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	moduleDir := filepath.Join(root, "go")
	writeCoverFixture(t, moduleDir)
	hold, err := verifylock.Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, passed := realGoUnitTest(ctx, moduleDir, "./p")

	if !passed || ctx.Err() != nil {
		t.Errorf("passed = %t, ctx = %v, output:\n%s\nwant the per-package unit run to finish while another lane holds the host lock", passed, ctx.Err(), out)
	}
}

func TestRecordUnserializedFloor_AnAbsentWorkspaceIsReportedAndNotCreated(t *testing.T) {
	t.Parallel()
	var errOut injectedStderr
	in := ReviewInput{Cycle: 7, Workspace: filepath.Join(t.TempDir(), "gone"), Stderr: &errOut}

	in.recordUnserializedFloor(t.TempDir(), context.DeadlineExceeded)

	if _, err := os.Stat(in.Workspace); !os.IsNotExist(err) || !strings.Contains(errOut.String(), "not recorded as an abnormal event") {
		t.Errorf("workspace stat = %v, stderr = %q; want no workspace created and a WARN that the event was not recorded", err, errOut.String())
	}
}
