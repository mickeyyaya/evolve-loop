package inboxmover

// root_failure.go — ADR-0080 P2: FAIL-side attempt accounting for
// ROOT-RESIDENT items. The ADR-0072 S5 bump+quarantine lives on the
// processing/-release path, which wave lanes never enter (items stay in the
// inbox root; only PASS promotes them) — so graded audit FAILs incremented
// nothing and the task-retry ceiling was structurally unreachable:
// workspace-hygiene burned 12 lanes, quarantine-dead 7, failure_count 0.
// RecordRootTaskFailure is the root-resident twin, called by the loop after
// a task-level cycle FAIL for each triage-COMMITTED id (menu semantics: an
// unworked menu id never bumps).
//
// CONCURRENCY (review HIGH): the root is SHARED — two lanes that committed
// the same id can FAIL concurrently, and an unserialized read-modify-write
// both loses attempts and can rename a stale copy back into the root after
// the other lane quarantined it (resurrection — the ceiling defeated in the
// exact contended case it exists for). All bumps therefore serialize on one
// inbox-level flock, and the item is re-resolved INSIDE the lock.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

// bumpLockTimeout bounds the wait for the inbox bump lock — accounting is
// best-effort; a wedged sibling must cost one WARN, never a hang.
const bumpLockTimeout = 30 * time.Second

// RecordRootTaskFailure bumps taskID's durable failure_count where the item
// lives (the inbox root) and, when a positive ceiling is met, moves it to the
// terminal quarantine/ dir. Unknown ids are a quiet no-op (scout-originated
// work has no inbox item); ceiling <= 0 disables quarantine (the policy
// escape hatch, same contract as ShouldQuarantine).
func RecordRootTaskFailure(opts Options, taskID string, cycle int, reason string, ceiling int) (count int, quarantined bool, err error) {
	opts.resolveOpts()
	ctx, cancel := context.WithTimeout(context.Background(), bumpLockTimeout)
	defer cancel()
	release, lerr := verifylock.AcquireAt(ctx, filepath.Join(opts.InboxDir, ".bump.lock"), opts.Stderr)
	if lerr != nil {
		return 0, false, fmt.Errorf("root-failure: bump lock: %w", lerr)
	}
	defer release()
	// Resolve INSIDE the lock: a concurrent lane may have just bumped or
	// quarantined this id — a stale pre-lock path must never be written back.
	path, err := findFileByTaskID(opts.InboxDir, taskID)
	if err != nil || path == "" {
		return 0, false, nil // not root-resident (unknown, or already quarantined)
	}
	count, err = bumpFailureCount(path, fmt.Sprintf("cycle-%d: %s", cycle, reason))
	if err != nil {
		return 0, false, fmt.Errorf("root-failure: bump %s: %w", taskID, err)
	}
	opts.logf("", "root-failure: %s failure_count=%d (cycle %d)", taskID, count, cycle)
	if !ShouldQuarantine(count, ceiling, false) {
		return count, false, nil
	}
	qDir := filepath.Join(opts.InboxDir, "quarantine")
	if err := os.MkdirAll(qDir, 0o755); err != nil {
		return count, false, fmt.Errorf("root-failure: quarantine dir: %w", err)
	}
	dest := filepath.Join(qDir, filepath.Base(path))
	if err := os.Rename(path, dest); err != nil {
		return count, false, fmt.Errorf("root-failure: quarantine move %s: %w", taskID, err)
	}
	opts.logf("", "root-failure: %s QUARANTINED at failure_count=%d (ceiling %d) — terminal until operator release", taskID, count, ceiling)
	return count, true, nil
}
