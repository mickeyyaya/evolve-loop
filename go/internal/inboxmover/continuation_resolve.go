package inboxmover

// continuation_resolve.go — ADR-0076 slice C, resolve side: the orchestrator's
// composition-root lookup for "does this cycle's claimed scope carry preserved
// work to resume?". Reads the cycle's processing claims (the same dir the
// claim/release lifecycle owns) and returns the first stamped continuation in
// deterministic filename order. Validation is the orchestrator's job
// (validateContinuation re-screens against live git state); this is pure
// tolerant lookup — any unreadable item is skipped.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// ResolveContinuation returns the first continuation stamped on this cycle's
// processing claims (deterministic filename order), or nil when none carries
// one.
func ResolveContinuation(opts Options, cycle int) *continuation.Continuation {
	return resolveClaim(opts, cycle, nil)
}

// resolveClaim walks this cycle's claimed items in sorted order and returns the
// first STAMPED one that inScope admits. inScope nil admits every claim — the
// legacy, scope-blind reading.
//
// The scope filter exists because a cycle's processing dir can hold claims for
// several ids, and a lane may legitimately be working only one of them. Without
// the filter the walk returns whichever stamped claim it meets first, which
// silently hands a lane a PEER LANE's continuation: cycle-1536, scoped to
// "pipeline-defect-infra-systemic", adopted "pipeline-replay-contract-boundary"
// (cycle-1535's task), shipped that lane's eval file, and left cycle-1535 unable
// to rebase — a PASS cycle whose landing was destroyed. Unstamped claims are
// skipped, so the lane's own claim not carrying a stamp is exactly the case that
// let the peer's win.
func resolveClaim(opts Options, cycle int, inScope map[string]bool) *continuation.Continuation {
	opts.resolveOpts()
	cycleDir := filepath.Join(opts.InboxDir, "processing", fmt.Sprintf("cycle-%d", cycle))
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

// scopeSet turns a lane's declared ids into a membership set, or nil when the
// lane declares none. nil means "no lane identity to violate" — a sequential or
// solo cycle — and preserves the legacy scope-blind reading exactly.
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

// ResolveContinuationForScope is the composition root's lookup once a cycle's
// scope identity can come from either class (ADR-0076 slice C, G2). Inbox
// claims are tried FIRST — G1's semantics are untouched, and a claim that
// carries a stamp always wins — then the scope-id-keyed registry over scopeIDs
// in the order the lane declares them (deterministic, so a re-run resolves the
// same binding). A claim merely EXISTING never suppresses the fallback; only a
// stamped one does. An entry with no snapshot ref is not resumable work, so it
// is not a binding. A corrupt registry degrades to nil (fresh start) with a
// loud line — the orchestrator must never crash mid-cycle over a salvage index.
func ResolveContinuationForScope(opts Options, cycle int, scopeIDs []string) *continuation.Continuation {
	// Claim-first (G1) is unchanged — but a SCOPED lane only adopts claims
	// belonging to its own scope. An unscoped cycle passes nil and reads exactly
	// as before.
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
			// Live-scope guard (planner-and-adoption-live-scope-guard). This is
			// the ONE seam both the wave planner's lane-scope minting and the
			// post-triage adoption path go through, and it used to trust a
			// registry hit without asking whether the scope id still names a
			// live pending item — so a parked/consumed scope re-armed on every
			// wave with no adoption event and no carryover entry (cycles 1487,
			// 1497). The belt holds even if a release call is ever missed at a
			// pool-exit path; releasing the ghost here stops it re-arming.
			//
			// Two conditions the audit of cycle-1507 (H1/H2) made mandatory
			// before this read path is allowed to DELETE a root-owned binding:
			//
			//  1. RECENCY. A retired copy older than the binding is not evidence
			//     of a ghost — the item was re-filed and rebound AFTER that
			//     retirement, and its archived copy sits in consumed/ forever.
			//     Releasing on that evidence destroys live preserved work
			//     (measured: 7 of 91 real bindings on the runtime plane).
			//     Unknown (0) is not stale — the ordinary quarantine copy
			//     carries no cycle stamp, and refusing to act on the common case
			//     would disarm the belt entirely.
			//  2. PRESERVE FIRST. The release goes through the ONE shared
			//     transaction (ReleaseContinuationBinding), so the salvage
			//     pointer lands in the item file before the delete instead of
			//     on stderr only.
			retiredPath, retiredIn := scopeRetiredAt(opts, id)
			if retiredIn != "" && !scopeHasLiveItem(opts, id) {
				if rc := retiredAtCycle(retiredPath); rc > 0 && rc < c.Cycle {
					fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d scope %q has a retired copy in inbox/%s/ from cycle %d, but its binding is NEWER (cycle %d) — the item was re-filed and rebound after that retirement, so the copy is stale evidence: adopting the binding, not releasing it\n", cycle, id, retiredIn, rc, c.Cycle)
					resolved := c
					return &resolved
				}
				fmt.Fprintf(opts.Stderr, "[inbox] WARN cycle %d refusing continuation binding for scope %q: its item is retired in inbox/%s/ with no live pending copy (not in the inbox root, not claimed in processing/) — releasing the dead binding (snapshot %s, cycle %d)\n", cycle, id, retiredIn, c.SnapshotSHA, c.Cycle)
				if _, _, derr := ReleaseContinuationBinding(opts, id, fmt.Sprintf("scope-read-guard-cycle-%d", cycle)); derr != nil {
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
