package main

import (
	"fmt"
	"io"
	"path/filepath"
)

type inboxItemFingerprintFields struct {
	ConsumedBy string `json:"consumed_by"`
	Notes      string `json:"notes"`
}

func runInboxAckFingerprint(root string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(stderr, inboxUsage("ack-fingerprint"))
		return 10
	}
	itemPath := args[0]
	evolveDir := filepath.Join(root, ".evolve")
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
