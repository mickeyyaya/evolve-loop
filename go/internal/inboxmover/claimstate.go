package inboxmover

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

type Location = lifecycle.Location

func Locate(inboxDir, id string) (Location, error) {
	return lifecycle.Locate(inboxDir, id)
}

func deliveredAt(inboxDir, id string) (string, bool) {
	path, err := FindFileByTaskID(filepath.Join(inboxDir, StateDelivered), id)
	return path, err == nil
}

func HeldBy(inboxDir, id string) (string, bool) {
	if claimDir, _, held := claimHolding(inboxDir, id); held {
		return "the claim of " + claimDir, true
	}
	if _, delivered := deliveredAt(inboxDir, id); delivered {
		return "its delivery to the integration", true
	}
	return "", false
}
