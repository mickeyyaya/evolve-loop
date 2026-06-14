package fleet

import "path/filepath"

// Todo is one unit of backlog work plus the repo files it would touch. The
// advisor supplies the file scope (from the task's plan / changed-file estimate).
type Todo struct {
	ID    string
	Files []string
}

// Partition assigns todos to n independent cycle buckets such that NO bucket
// holds two todos that touch the same repo file — so the cycles can run
// concurrently without colliding on the shared tree (ADR-0049 E: the advisor's
// "plan, separate and assign the todos to independent cycles"). It is a greedy
// first-fit by normalized file ownership: each todo goes to the first bucket it
// is disjoint from. A todo that conflicts with EVERY bucket is returned in
// `deferred` rather than co-scheduled with conflicting work — the caller runs it
// in a later wave (never silently overlapping the shared tree). Determinism: the
// input order is preserved, so the partition is reproducible (no map iteration).
func Partition(todos []Todo, n int) (buckets [][]Todo, deferred []Todo) {
	if n < 1 {
		n = 1
	}
	buckets = make([][]Todo, n)
	claimed := make([]map[string]bool, n) // normalized files owned by each bucket
	for i := range claimed {
		claimed[i] = map[string]bool{}
	}
	for _, td := range todos {
		files := normalizeFiles(td.Files)
		placed := false
		for b := 0; b < n; b++ {
			if disjoint(claimed[b], files) {
				buckets[b] = append(buckets[b], td)
				for f := range files {
					claimed[b][f] = true
				}
				placed = true
				break
			}
		}
		if !placed {
			deferred = append(deferred, td)
		}
	}
	return buckets, deferred
}

// normalizeFiles returns the todo's files as a normalized set so two spellings
// of the same repo file (./a.go vs a.go) collide. Mirrors the intent of
// swarm.normalizePath (unexported there); a file-conflict check only needs
// filepath.Clean.
func normalizeFiles(files []string) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[filepath.Clean(f)] = true
	}
	return out
}

// disjoint reports whether none of files is already owned by the bucket.
func disjoint(owned map[string]bool, files map[string]bool) bool {
	for f := range files {
		if owned[f] {
			return false
		}
	}
	return true
}
