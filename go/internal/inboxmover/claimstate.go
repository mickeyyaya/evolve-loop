package inboxmover

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"

// Location is where an inbox item lives; Cycle is 0 at the root, else the claiming cycle.
type Location = lifecycle.Location

// Locate resolves an id to its claim, then its root copy, else ErrNotFound; only a read fault is returned as itself.
func Locate(inboxDir, id string) (Location, error) {
	return lifecycle.Locate(inboxDir, id)
}
