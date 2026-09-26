package inboxmover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

// bumpLockTimeout bounds the lock wait: accounting is best-effort, so a wedged sibling costs an error, never a hang.
const bumpLockTimeout = 30 * time.Second

// RecordRootTaskFailure bumps a root item's failure_count and quarantines it at a positive ceiling; unknown ids are a no-op.
func RecordRootTaskFailure(opts Options, taskID string, cycle int, reason string, ceiling int) (count int, quarantined bool, err error) {
	opts.resolveOpts()
	ctx, cancel := context.WithTimeout(context.Background(), bumpLockTimeout)
	defer cancel()
	// The root is shared: concurrent lanes' read-modify-writes lose attempts or resurrect a quarantined item.
	release, lerr := verifylock.AcquireAt(ctx, filepath.Join(opts.InboxDir, ".bump.lock"), opts.Stderr)
	if lerr != nil {
		return 0, false, fmt.Errorf("root-failure: bump lock: %w", lerr)
	}
	defer release()
	// Resolve inside the lock, so a path another lane just moved is never written back.
	path, err := FindFileByTaskID(opts.InboxDir, taskID)
	if err != nil || path == "" {
		return 0, false, nil
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
