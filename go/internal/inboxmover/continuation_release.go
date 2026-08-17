package inboxmover

// continuation_release.go — the ONE release transaction every retirement path
// shares (cycle-1515). `releaseContinuationOnRetire` already owned the
// preserve-then-delete order for the write side (Promote/quarantine); the
// read-side guard in ResolveContinuationForScope and the new operator surface
// (`evolve continuation release`) both need the SAME transaction, and a second
// copy of it is exactly the drift that produced audit cycle-1507's H2 (the
// read-side delete skipped preservation and sent the salvage pointer to stderr
// only). So the transaction lives here once and the three callers reach it.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// ReleaseContinuationBinding releases scopeID's registry binding, preserving the
// released VALUE into the scope's item file FIRST (wherever that item currently
// lives — pending root, a processing claim, or a retirement subtree). Returns
// the released value, whether the registry entry was actually deleted, and any
// registry error.
//
// A scope with no binding is a clean miss (zero, false, nil) — releasing
// nothing is not a failure. The delete is DeleteRegistryEntryIfCycle, so a
// sibling lane that rebound the scope between the read and the delete keeps its
// fresh binding (released=false, no error).
func ReleaseContinuationBinding(opts Options, scopeID, reason string) (continuation.Continuation, bool, error) {
	opts.resolveOpts()
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
		if perr := appendReleasedContinuation(path, continuation.RedactHostPaths(c), reason, opts.Now().UTC()); perr != nil {
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

// FindScopeItemFile returns the path of the inbox item carrying scopeID, or ""
// when no copy exists anywhere. Search order is liveness order — the pending
// root, then a processing claim, then the retirement subtrees — so the
// preserved pointer always lands on the copy an operator would actually open.
func FindScopeItemFile(opts Options, scopeID string) string {
	opts.resolveOpts()
	if strings.TrimSpace(scopeID) == "" {
		return ""
	}
	if p, err := FindFileByTaskID(opts.InboxDir, scopeID); err == nil {
		return p
	}
	if entries, err := os.ReadDir(filepath.Join(opts.InboxDir, "processing")); err == nil {
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), "cycle-") {
				continue
			}
			if p, ferr := FindFileByTaskID(filepath.Join(opts.InboxDir, "processing", e.Name()), scopeID); ferr == nil {
				return p
			}
		}
	}
	path, _ := scopeRetiredAt(opts, scopeID)
	return path
}

// retiredAtCycle returns the cycle a retired item copy was retired in, or 0 when
// the copy carries no cycle evidence at all.
//
// This is the recency half of the read-side guard's evidence test (audit
// cycle-1507 H1): a retired copy from cycle 900 says NOTHING about a binding
// minted at cycle 1484 — the item was re-filed and rebound after that
// retirement, and treating the old copy as proof of death releases live
// preserved work. Only a retirement that is not older than the binding is
// evidence of a ghost. Unknown (0) is not "stale": the ordinary quarantine copy
// carries no stamp, and refusing to act on the common case would disarm the
// belt entirely.
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

// cycleOf coerces a cycle field that the inbox schema writes as either a JSON
// number or a string ("1515") into an int; anything else is 0 (unknown).
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

// cycleFromDirName parses the "cycle-N" level promoteDestPath nests under
// processed/ and rejected/.
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
