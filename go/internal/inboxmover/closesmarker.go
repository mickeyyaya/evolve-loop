package inboxmover

import "github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"

func ClosesInboxIDs(body []byte) []string { return closesinbox.IDs(body) }
