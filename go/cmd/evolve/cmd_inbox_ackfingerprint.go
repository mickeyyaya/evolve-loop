package main

import (
	"fmt"
	"io"
	"path/filepath"
)

// inboxItemFingerprintFields is the minimal subset of an inbox item's JSON
// shape this subcommand needs — consumed_by (the narrative written when an
// item moves to .evolve/inbox/consumed/) and notes (the auto-filed field
// present before a narrative exists).
type inboxItemFingerprintFields struct {
	ConsumedBy string `json:"consumed_by"`
	Notes      string `json:"notes"`
}

func runInboxAckFingerprint(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: evolve inbox ack-fingerprint <item-path>")
		return 10
	}
	itemPath := args[0]
	// Shares ackItemFingerprint with the reconciler and `evolve inbox consume`
	// (cmd_inbox_consume.go) — one extraction path, one ledger writer.
	evolveDir := filepath.Join(envOrCwd("EVOLVE_PROJECT_ROOT"), ".evolve")
	fp, found, err := ackItemFingerprint(evolveDir, itemPath, "inbox-consumption")
	if err != nil {
		fmt.Fprintf(stderr, "inbox ack-fingerprint: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "inbox ack-fingerprint: %s: no fingerprint token found in consumed_by or notes\n", itemPath)
		return 1
	}
	fmt.Fprintf(stdout, "inbox ack-fingerprint: acknowledged %q in resolved-fingerprints.json — blocker-breaker will exclude it going forward\n", fp)
	return 0
}
