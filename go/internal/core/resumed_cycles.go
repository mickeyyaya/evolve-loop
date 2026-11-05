package core

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func ResumedCycleIDs(root string, from, to time.Time) ([]int, []error) {
	path := filepath.Join(paths.EvolveDirOf(root), signalcenter.StreamFileName)
	chunk, err := signalcenter.ReadStream(path, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}
	var failures []error
	if chunk.Skipped > 0 {
		failures = append(failures, fmt.Errorf("skipped %d malformed signal line(s) in %s", chunk.Skipped, path))
	}
	seen := map[int]bool{}
	for _, e := range chunk.Events {
		if e.Code != CodeLaneResumed {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, e.TS)
		if !at.Before(from) && (to.IsZero() || at.Before(to)) {
			seen[e.Cycle] = true
		}
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids, failures
}
