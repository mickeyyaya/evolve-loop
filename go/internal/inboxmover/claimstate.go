package inboxmover

// claimstate.go — the read-only view of where an item stands in the claim
// lifecycle. Locate is the ONE walk over the inbox root and processing/cycle-*/
// (Promote's source resolution, the continuation scope readers and the
// ADR-0100 declared-effects gate all go through it), and it derives the layout
// from inboxbatch exactly as the writer, Claim, does — so no reader can drift
// from where a claim actually lands. The walk lives in the lifecycle leaf
// since ADR-0103 unit 06; this file keeps the spelling.

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"

// Location is where an inbox item currently lives. Cycle is 0 while the item
// is pending at the inbox root and the claiming cycle once it sits under
// processing/cycle-<Cycle>/.
type Location = lifecycle.Location

// Locate resolves an item id to its file — a processing claim first, then the
// pending root; an id with no file in either place is ErrNotFound; only a read
// fault on an existing directory is returned as itself.
func Locate(inboxDir, id string) (Location, error) {
	return lifecycle.Locate(inboxDir, id)
}
