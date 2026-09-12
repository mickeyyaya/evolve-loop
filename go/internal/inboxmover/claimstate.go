package inboxmover

// claimstate.go — the read-only view of where an item stands in the claim
// lifecycle. Locate is the ONE walk over the inbox root and processing/cycle-*/
// (Promote's source resolution, the continuation scope readers and the
// ADR-0100 declared-effects gate all go through it), and it derives the layout
// from inboxbatch exactly as the writer, Claim, does — so no reader can drift
// from where a claim actually lands.

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// Location is where an inbox item currently lives. Cycle is 0 while the item
// is pending at the inbox root and the claiming cycle once it sits under
// processing/cycle-<Cycle>/.
type Location struct {
	Path  string
	Cycle int
}

// Locate resolves an item id to its file. Liveness order is Promote's: a
// processing claim first (a lane holding the item outranks a stale root copy
// of the same id), then the pending root. An id with no file in either place —
// including a project with no inbox at all — is ErrNotFound; only a read fault
// on an existing directory is returned as itself.
//
// The two reads are not one atomic snapshot: a rename of this very id landing
// between them (a sibling lane claiming it mid-scan) can read as ErrNotFound
// once. Accepted: every caller re-reads on its next step (the gate's
// correction ladder re-verifies, Promote re-resolves), and a false "absent"
// never fails anything closed.
func Locate(inboxDir, id string) (Location, error) {
	for _, dir := range inboxbatch.ProcessingCycleDirs(inboxDir) {
		if path, ferr := FindFileByTaskID(dir, id); ferr == nil {
			cycle, _ := inboxbatch.ParseProcessingCycle(filepath.Base(dir))
			return Location{Path: path, Cycle: cycle}, nil
		}
	}
	path, err := FindFileByTaskID(inboxDir, id)
	switch {
	case err == nil:
		return Location{Path: path}, nil
	case errors.Is(err, ErrNotFound) || os.IsNotExist(err):
		return Location{}, ErrNotFound
	default:
		return Location{}, err
	}
}
