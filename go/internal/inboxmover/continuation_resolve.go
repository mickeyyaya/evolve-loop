package inboxmover

import (
	"encoding/json"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// ResolveContinuation returns the first stamped continuation among this cycle's claims, in filename order, or nil.
func ResolveContinuation(opts Options, cycle int) *continuation.Continuation {
	return resolveClaim(opts, cycle, nil)
}

// resolveClaim returns the first stamped claim that inScope admits; nil admits all.
// A scoped lane must skip a peer lane's claim, or two lanes work one task.
func resolveClaim(opts Options, cycle int, inScope map[string]bool) *continuation.Continuation {
	opts.resolveOpts()
	cycleDir := inboxbatch.ProcessingCycleDir(opts.InboxDir, cycle)
	entries, err := os.ReadDir(cycleDir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		body, rerr := os.ReadFile(filepath.Join(cycleDir, name))
		if rerr != nil {
			continue
		}
		var it struct {
			ID           string                     `json:"id"`
			Continuation *continuation.Continuation `json:"continuation"`
		}
		if json.Unmarshal(body, &it) != nil {
			continue
		}
		if it.Continuation == nil || it.Continuation.SnapshotSHA == "" {
			continue
		}
		if inScope != nil && !inScope[strings.TrimSpace(it.ID)] {
			fmt.Fprintf(opts.Stderr, "[inbox] cycle %d skipping claim %q: it carries a continuation from cycle %d but is OUTSIDE this lane's declared scope — adopting it would make two lanes work one task\n",
				cycle, it.ID, it.Continuation.Cycle)
			continue
		}
		return it.Continuation
	}
	return nil
}

// scopeSet returns nil when the lane declares no ids, keeping a solo cycle scope-blind.
func scopeSet(scopeIDs []string) map[string]bool {
	set := map[string]bool{}
	for _, id := range scopeIDs {
		if id = strings.TrimSpace(id); id != "" {
			set[id] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// ResolveContinuationForScope tries this lane's stamped claims, then the registry for scopeIDs in order.
func ResolveContinuationForScope(opts Options, cycle int, scopeIDs []string) *continuation.Continuation {
	opts.resolveOpts()
	if c := resolveClaim(opts, cycle, scopeSet(scopeIDs)); c != nil {
		return c
	}
	for _, id := range scopeIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		c, ok, err := continuation.ReadRegistryEntry(opts.ProjectRoot, id)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d continuation registry unreadable (%v) — no lane-scope binding resolved\n", cycle, err)
			return nil
		}
		if ok && c.SnapshotSHA != "" {
			// A binding whose scope is retired, with no live copy, is a ghost: release it so it
			// stops re-arming, unless the retired copy is older than the binding.
			retiredPath, retiredIn := scopeRetiredAt(opts, id)
			if retiredIn != "" && !scopeHasLiveItem(opts, id) {
				if rc := retiredAtCycle(retiredPath); rc > 0 && rc < c.Cycle {
					fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d scope %q has a retired copy in inbox/%s/ from cycle %d, but its binding is NEWER (cycle %d) — the item was re-filed and rebound after that retirement, so the copy is stale evidence: adopting the binding, not releasing it\n", cycle, id, retiredIn, rc, c.Cycle)
					resolved := c
					return &resolved
				}
				fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d refusing continuation binding for scope %q: its item is retired in inbox/%s/ with no live pending copy (not in the inbox root, not claimed in processing/) — releasing the dead binding (snapshot %s, cycle %d)\n", cycle, id, retiredIn, c.SnapshotSHA, c.Cycle)
				if _, _, derr := ReleaseContinuationBinding(opts, id, fmt.Sprintf("scope-read-guard-cycle-%d", cycle), "runtime (scope read guard)"); derr != nil {
					fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d could not release dead binding for scope %q: %v\n", cycle, id, derr)
				}
				continue
			}
			resolved := c
			return &resolved
		}
	}
	return nil
}
