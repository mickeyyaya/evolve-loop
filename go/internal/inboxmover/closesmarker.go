package inboxmover

import "github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"

func ClosesInboxIDs(body []byte) []string { return closesinbox.IDs(body) }

func RefsInboxIDs(body []byte) []string { return closesinbox.RefIDs(body) }
