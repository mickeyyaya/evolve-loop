// cmd_inbox_ackfingerprint.go — `evolve inbox ack-fingerprint <item-path>`,
// the transactional-consumption counterpart to cycle-1332's manual
// `evolve loop --reset --fingerprint <fp>`. Reads a consumed pipeline-defect
// inbox item's consumed_by/notes fields, extracts the failure fingerprint,
// and acks it into .evolve/resolved-fingerprints.json via
// core.ConsumePipelineDefectFingerprint — closing the gap the inbox item
// 2026-08-05T09-40-00Z-recurrence-ack-for-consumed-p0.json named: the
// operator otherwise has to hand-retype the fingerprint into --reset
// --fingerprint after every consumption.
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
