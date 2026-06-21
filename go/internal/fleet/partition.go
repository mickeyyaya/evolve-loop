package fleet

import (
	"path/filepath"
	"strings"
)

// PlanCycles partitions a backlog into at most `count` concurrent cycle specs,
// each scoped to a DISJOINT set of todos, plus the deferred todos that could not
// be co-scheduled this wave (run them in a later wave). It is the single adapter
// from the advisor's backlog to fleet's launch specs (ADR-0049 E): each spec
// carries its todo IDs in Scope and in Env[fleetScopeEnvKey] (comma-joined) so
// the launched cycle's triage selects only its subset. Empty buckets yield NO
// spec — when the backlog can't fill `count` cycles, fewer launch (not idle ones).
func PlanCycles(todos []Todo, count int) (specs []CycleSpec, deferred []Todo) {
	buckets, deferred := Partition(todos, count)
	for _, b := range buckets {
		if len(b) == 0 {
			continue
		}
		ids := make([]string, len(b))
		for i, td := range b {
			ids[i] = td.ID
		}
		specs = append(specs, CycleSpec{
			Scope: ids,
			Env:   map[string]string{fleetScopeEnvKey: strings.Join(ids, ",")},
		})
	}
	return specs, deferred
}

// Todo is one unit of backlog work plus the repo files it would touch. The
// advisor (or the preliminary-study phase) supplies the file scope. JSON tags
// match the `evolve fleet --plan` backlog and the richer campaign-plan.json.
type Todo struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
	// DependsOn lists todo IDs that must complete before this todo's cycle runs —
	// the cross-cycle DAG that orders the campaign into waves. Empty = no prereqs.
	DependsOn []string `json:"depends_on,omitempty"`
	// Priority orders todos when scheduling is budget-limited (higher runs first);
	// advisory — it never affects dependency correctness.
	Priority int `json:"priority,omitempty"`
	// Optional marks a best-effort cycle: if it still fails after retries the
	// campaign QUARANTINES it (records the id, continues) instead of aborting. A
	// required (Optional=false) cycle's exhausted failure aborts the wave, since
	// its dependents cannot safely proceed. Default false = required.
	Optional bool `json:"optional,omitempty"`
	// OutputContract is the one-line done-definition the launched cycle must
	// satisfy — the explicit per-cycle objective that prevents duplicated/missed
	// work (2026 multi-agent lesson). Carried through to the cycle.
	OutputContract string `json:"output_contract,omitempty"`
	// ToolScope optionally constrains the tools the launched cycle may use.
	ToolScope []string `json:"tool_scope,omitempty"`
}

// Partition assigns todos to n concurrent cycle buckets such that every repo
// file is owned by AT MOST ONE bucket (ADR-0049 E: the advisor's "plan, separate
// and assign the todos to independent cycles"). Each bucket runs as its OWN
// concurrent `evolve cycle run` in its own worktree, so the load-bearing
// invariant is CROSS-bucket disjointness: a file appearing in two buckets means
// two cycles edit it at once and collide on the shared tree at ship time. Two
// todos touching one file therefore CLUSTER into the same bucket (one cycle, one
// worktree, sequential — safe), never spread across buckets.
//
// Greedy file-ownership pass (deterministic — input order preserved, ties break
// to the lowest bucket index, no map-iteration nondeterminism):
//   - a todo whose files are all unclaimed → the least-loaded bucket (spreads
//     independent work across cycles);
//   - a todo overlapping exactly ONE bucket's files → joins that bucket;
//   - a todo whose files are split across ≥2 buckets would bridge two concurrent
//     cycles into a collision, so it is DEFERRED to a later wave rather than
//     co-scheduled (the caller runs deferred todos once the current wave lands).
func Partition(todos []Todo, n int) (buckets [][]Todo, deferred []Todo) {
	if n < 1 {
		n = 1
	}
	buckets = make([][]Todo, n)
	owner := map[string]int{} // normalized file -> the one bucket that owns it
	for _, td := range todos {
		files := normalizeFiles(td.Files)
		owning := owningBuckets(owner, files)
		switch len(owning) {
		case 0:
			// All files free: spread to the least-loaded bucket.
			place(buckets, owner, leastLoaded(buckets), td, files)
		case 1:
			// Overlaps exactly one bucket: join it (same cycle, sequential — safe).
			place(buckets, owner, only(owning), td, files)
		default:
			// Files split across ≥2 concurrent buckets — cannot isolate without
			// bridging them. Defer rather than co-schedule a cross-tree collision.
			deferred = append(deferred, td)
		}
	}
	return buckets, deferred
}

// owningBuckets returns the set of bucket indices that already own any of files.
func owningBuckets(owner map[string]int, files map[string]bool) map[int]bool {
	out := map[int]bool{}
	for f := range files {
		if b, ok := owner[f]; ok {
			out[b] = true
		}
	}
	return out
}

// place adds td to bucket b and claims every one of td's files for b (including
// previously-free ones, so the file can never later be claimed by another bucket).
func place(buckets [][]Todo, owner map[string]int, b int, td Todo, files map[string]bool) {
	buckets[b] = append(buckets[b], td)
	for f := range files {
		owner[f] = b
	}
}

// leastLoaded returns the index of the bucket with the fewest todos (lowest
// index on ties) — keeps independent work spread across cycles.
func leastLoaded(buckets [][]Todo) int {
	best, bestN := 0, -1
	for i, b := range buckets {
		if bestN == -1 || len(b) < bestN {
			best, bestN = i, len(b)
		}
	}
	return best
}

// only returns the single key of a one-element set. Called only from the case-1
// branch (len==1); an empty set is a caller-contract violation — fail loud.
func only(set map[int]bool) int {
	for k := range set {
		return k
	}
	panic("fleet.only: called on empty set")
}

// normalizeFiles returns the todo's files as a normalized set so two spellings
// of the same repo file (./a.go vs a.go) collide. filepath.Clean is enough for a
// file-conflict check.
func normalizeFiles(files []string) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[filepath.Clean(f)] = true
	}
	return out
}
