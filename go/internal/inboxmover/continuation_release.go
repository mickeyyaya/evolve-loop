package inboxmover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func ReleaseContinuationBinding(opts Options, scopeID, reason, releasedBy string) (continuation.Continuation, bool, error) {
	opts.resolveOpts()
	if err := cycleWorktreeRefusal(opts.InboxDir); err != nil {
		return continuation.Continuation{}, false, err
	}
	if strings.TrimSpace(scopeID) == "" {
		return continuation.Continuation{}, false, fmt.Errorf("inboxmover: empty scope id is not releasable")
	}
	c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, scopeID)
	if err != nil {
		return continuation.Continuation{}, false, fmt.Errorf("inboxmover: continuation registry unreadable while releasing %q: %w", scopeID, err)
	}
	if !ok {
		return continuation.Continuation{}, false, nil
	}
	if path := FindScopeItemFile(opts, scopeID); path != "" {
		if perr := appendReleasedContinuation(path, continuation.RedactHostPaths(c), reason, releasedBy, opts.Now().UTC()); perr != nil {
			opts.logf("WARN: ", "release '%s': preserved pointer (snapshot %s) NOT written to %s: %v — releasing the binding anyway", scopeID, c.SnapshotSHA, path, perr)
		}
	} else {
		opts.logf("WARN: ", "release '%s': no item file found to preserve the pointer into (snapshot %s, branch %s, base %s, cycle %d) — recording it on this line only", scopeID, c.SnapshotSHA, c.Branch, c.BaseSHA, c.Cycle)
	}
	released, derr := continuation.DeleteRegistryEntryIfCycle(opts.ProjectRoot, scopeID, c.Cycle)
	if derr != nil {
		return c, false, fmt.Errorf("inboxmover: release %q: %w", scopeID, derr)
	}
	return c, released, nil
}

func FindScopeItemFile(opts Options, scopeID string) string {
	opts.resolveOpts()
	if strings.TrimSpace(scopeID) == "" {
		return ""
	}
	if loc, err := Locate(opts.InboxDir, scopeID); err == nil {
		return loc.Path
	}
	path, _ := scopeRetiredAt(opts, scopeID)
	return path
}

func retiredAtCycle(path string) int {
	if path == "" {
		return 0
	}
	if n := cycleFromDirName(filepath.Base(filepath.Dir(path))); n > 0 {
		return n
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var doc struct {
		Cycle    any `json:"cycle"`
		Consumed struct {
			Cycle any `json:"cycle"`
		} `json:"consumed"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return 0
	}
	if n := cycleOf(doc.Cycle); n > 0 {
		return n
	}
	return cycleOf(doc.Consumed.Cycle)
}

func cycleOf(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func cycleFromDirName(name string) int {
	if !strings.HasPrefix(name, "cycle-") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
	if err != nil {
		return 0
	}
	return n
}

func ReconcileConsumedBindings(opts Options) (released []string) {
	opts.resolveOpts()
	dir := filepath.Join(opts.InboxDir, "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		id := itemIDAt(path)
		if id == "" {
			continue
		}
		c, ok, rerr := continuation.ReadRegistryEntry(opts.ProjectRoot, id)
		if rerr != nil {
			opts.logf("WARN: ", "consumed-reconcile %q: registry unreadable (%v) — binding left in place", id, rerr)
			continue
		}
		if !ok {
			continue
		}
		if scopeHasLiveItem(opts, id) {
			continue
		}
		if rc := retiredAtCycle(path); rc > 0 && rc < c.Cycle {
			opts.logf("WARN: ", "consumed copy of %q is from cycle %d but its binding is NEWER (cycle %d) — stale evidence, not releasing (cycle-1507 recency guard)", id, rc, c.Cycle)
			continue
		}
		if _, rel, relErr := ReleaseContinuationBinding(opts, id, "consumed-reconcile", "runtime (consumed-corpus reconciler)"); relErr != nil {
			opts.logf("WARN: ", "consumed-reconcile release %q: %v", id, relErr)
		} else if rel {
			released = append(released, id)
		}
	}
	return released
}
