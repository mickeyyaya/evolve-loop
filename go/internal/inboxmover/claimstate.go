package inboxmover

import "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"

type Location = lifecycle.Location

func Locate(inboxDir, id string) (Location, error) {
	return lifecycle.Locate(inboxDir, id)
}
