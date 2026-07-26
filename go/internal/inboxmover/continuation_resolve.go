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
			Continuation *continuation.Continuation `json:"continuation"`
		}
		if json.Unmarshal(body, &it) == nil && it.Continuation != nil && it.Continuation.SnapshotSHA != "" {
			return it.Continuation
		}
	}
	return nil
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
	if c := ResolveContinuation(opts, cycle); c != nil {
		return c
	}
	opts.resolveOpts()
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
			resolved := c
			return &resolved
		}
	}
	return nil
}
