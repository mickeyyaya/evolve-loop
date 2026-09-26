package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// ackItemFingerprint reads one inbox item, extracts the failure fingerprint
// its consumed_by narrative (else its auto-filed notes) carries, and acks it
// into .evolve/resolved-fingerprints.json.
//
// found=false with a nil error is the routine case: the item carries no
// parseable fingerprint, which is not a failure — most inbox items are
// features, not pipeline defects. Errors are reserved for an item that could
// not be read or unmarshalled.
//
// An already-acked fingerprint is reported found and skipped: the ledger is a
// SET, so re-sweeping the consumed corpus on every breaker check must not grow
// it without bound.
func ackItemFingerprint(evolveDir, itemPath, resolvedBy string) (fingerprint string, found bool, err error) {
	raw, err := os.ReadFile(itemPath)
	if err != nil {
		return "", false, err
	}
	var item inboxItemFingerprintFields
	if err := json.Unmarshal(raw, &item); err != nil {
		return "", false, fmt.Errorf("%s: %w", itemPath, err)
	}
	fp, ok := core.ParseConsumptionFingerprint(item.ConsumedBy)
	if !ok {
		fp, ok = core.ParseConsumptionFingerprint(item.Notes)
	}
	if !ok {
		return "", false, nil
	}
	acked, lerr := core.LoadResolvedFingerprints(evolveDir)
	if lerr != nil {
		return "", false, lerr
	}
	if acked[fp] {
		return fp, true, nil
	}
	if _, cerr := core.ConsumePipelineDefectFingerprint(evolveDir, item.ConsumedBy, item.Notes, resolvedBy, time.Now().UTC()); cerr != nil {
		return "", false, cerr
	}
	return fp, true, nil
}

// reconcileConsumedFingerprints projects .evolve/inbox/consumed/ into the ack
// ledger: every consumed item whose narrative names a fingerprint is acked.
//
// Fail-loud-but-never-block: a per-item error WARNs by name and the sweep
// continues to its neighbours. This runs on the breaker's boot path, so a
// reconciler defect must never become a NEW pipeline blocker — the exact
// failure class this wiring exists to remove. An absent consumed/ directory is
// the normal case on a fresh tree and is silent.
func reconcileConsumedFingerprints(evolveDir string, stderr io.Writer) {
	dir := filepath.Join(evolveDir, "inbox", "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if _, _, aerr := ackItemFingerprint(evolveDir, filepath.Join(dir, e.Name()), "consumed-reconcile"); aerr != nil {
			fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: skipped unreadable consumed item %s: %v\n", e.Name(), aerr)
		}
	}
}

// runInboxConsume implements `evolve inbox consume <item-path>`: move the item
// into .evolve/inbox/consumed/ and ack any fingerprint it names, in one
// invocation. The move lands FIRST — a move that succeeded with a failed ack
// is repaired by reconcileConsumedFingerprints on the next breaker check,
// whereas an ack whose move failed would leave the item drawable by a lane
// while its fingerprint is already excused.
func runInboxConsume(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: evolve inbox consume <item-path>")
		return 10
	}
	itemPath := args[0]
	if _, err := os.Stat(itemPath); err != nil {
		fmt.Fprintf(stderr, "inbox consume: %v\n", err)
		return 1
	}
	evolveDir := filepath.Join(envOrCwd("EVOLVE_PROJECT_ROOT"), ".evolve")
	consumedDir := filepath.Join(evolveDir, "inbox", "consumed")
	if err := os.MkdirAll(consumedDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "inbox consume: %s: %v\n", itemPath, err)
		return 1
	}
	dest := filepath.Join(consumedDir, filepath.Base(itemPath))
	if err := os.Rename(itemPath, dest); err != nil {
		fmt.Fprintf(stderr, "inbox consume: %s: %v\n", itemPath, err)
		return 1
	}
	if releaseConsumedItemBinding(filepath.Dir(evolveDir), dest, stderr) {
		fmt.Fprintf(stdout, "inbox consume: released continuation binding for %s (salvage pointer preserved on the item)\n", filepath.Base(itemPath))
	}
	fp, found, err := ackItemFingerprint(evolveDir, dest, "inbox-consume")
	if err != nil {
		fmt.Fprintf(stderr, "inbox consume: %s consumed, but the fingerprint ack failed: %v\n", dest, err)
		return 1
	}
	if !found {
		fmt.Fprintf(stdout, "inbox consume: %s -> inbox/consumed/ (no fingerprint named; nothing to ack)\n", filepath.Base(itemPath))
		return 0
	}
	fmt.Fprintf(stdout, "inbox consume: %s -> inbox/consumed/ and acknowledged %q in resolved-fingerprints.json — blocker-breaker will exclude it going forward\n", filepath.Base(itemPath), fp)
	return 0
}

// releaseConsumedItemBinding releases a just-consumed item's continuation
// binding via the shared inboxmover.ReleaseContinuationBinding transaction. A
// direct consume needs no recency guard (it is an explicit operator statement
// the work is closed); the sweep below (reconcileConsumedBindings) carries
// that guard for items consumed by other routes.
func releaseConsumedItemBinding(projectRoot, itemPath string, stderr io.Writer) bool {
	id := ""
	if raw, err := os.ReadFile(itemPath); err == nil {
		var doc struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &doc) == nil {
			id = doc.ID
		}
	}
	if id == "" {
		return false
	}
	_, released, err := inboxmover.ReleaseContinuationBinding(
		inboxmover.Options{ProjectRoot: projectRoot, Stderr: stderr}, id, "inbox-consume", "operator (evolve inbox consume)")
	if err != nil {
		fmt.Fprintf(stderr, "[inbox] WARN: binding release %q: %v\n", id, err)
		return false
	}
	return released
}

// reconcileConsumedBindings delegates to the canonical consumed-corpus sweep
// (inboxmover.ReconcileConsumedBindings — live-copy + recency guards, shared
// release transaction). Runs beside reconcileConsumedFingerprints on the
// blocker-breaker path, i.e. before EVERY cycle dispatch, not only at boot.
func reconcileConsumedBindings(projectRoot, evolveDir string, stderr io.Writer) {
	released := inboxmover.ReconcileConsumedBindings(inboxmover.Options{ProjectRoot: projectRoot, Stderr: stderr})
	for _, id := range released {
		fmt.Fprintf(stderr, "[loop] blocker-breaker: released stray continuation binding for consumed item %s\n", id)
	}
	_ = evolveDir
}
