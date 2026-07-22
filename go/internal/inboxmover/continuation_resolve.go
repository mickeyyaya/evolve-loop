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
