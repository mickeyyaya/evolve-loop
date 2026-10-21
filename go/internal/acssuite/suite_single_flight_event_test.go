package acssuite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestAcquireSuiteLock_ALostSingleFlightIsAnAbnormalEventBesideTheVerdict(t *testing.T) {
	root := t.TempDir()
	hold, err := verifylock.Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	saved := suiteLockWait
	suiteLockWait = 200 * time.Millisecond
	defer func() { suiteLockWait = saved }()
	lock, _ := verifylock.LockPath(root)
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", "cycle-7"), 0o755); err != nil {
		t.Fatal(err)
	}

	acquireSuiteLock(Options{Root: root, ProjectRoot: root, Cycle: 7})()

	raw, _ := os.ReadFile(filepath.Join(root, ".evolve", "runs", "cycle-7", "abnormal-events.jsonl"))
	if !strings.Contains(string(raw), `"verify-single-flight-unavailable"`) || !strings.Contains(string(raw), lock) || !strings.Contains(string(raw), `"source_phase":"acs-suite"`) {
		t.Errorf("abnormal events = %q; want the unserialized suite run recorded with the lock path %s", raw, lock)
	}
}

func TestRecordUnserializedSuite_AnAbsentRunDirectoryIsReportedAndNotCreated(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-7")

	stderr := captureStderrF4(t, func() {
		recordUnserializedSuite(Options{Root: root, ProjectRoot: root, Cycle: 7}, context.DeadlineExceeded)
	})

	if _, err := os.Stat(runDir); !os.IsNotExist(err) || !strings.Contains(stderr, "not recorded as an abnormal event") {
		t.Errorf("run dir stat = %v, stderr = %q; want no directory created and a WARN that the event was not recorded", err, stderr)
	}
}
