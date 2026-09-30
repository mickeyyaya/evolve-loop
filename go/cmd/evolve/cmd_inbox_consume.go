package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
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

const inboxConsumeUsage = "usage: evolve inbox consume <item-path> [--resolution <text>] [--cycle <n>|console]"

type inboxConsumedStamp struct {
	At         string `json:"at"`
	Via        string `json:"via"`
	Cycle      string `json:"cycle"`
	Resolution string `json:"resolution"`
}

// runInboxConsume implements `evolve inbox consume <item-path> [--resolution
// <text>] [--cycle <n>]`: move the item into .evolve/inbox/consumed/, stamp it
// consumed{at, via, cycle, resolution}, and ack any fingerprint it names, in
// one invocation. The item is decoded BEFORE the move, so a malformed item is
// refused while still pending. The move lands FIRST — a move that succeeded
// with a failed stamp or ack is reported non-zero and the ack is repaired by
// reconcileConsumedFingerprints on the next breaker check, whereas an ack
// whose move failed would leave the item drawable by a lane while its
// fingerprint is already excused.
func runInboxConsume(args []string, stdout, stderr io.Writer) int {
	itemPath, stamp, ok := parseInboxConsumeArgs(args, stderr)
	if !ok {
		return 10
	}
	if _, err := os.Stat(itemPath); err != nil {
		fmt.Fprintf(stderr, "inbox consume: %v\n", err)
		return 1
	}
	doc, err := readInboxItemObject(itemPath)
	if err != nil {
		fmt.Fprintf(stderr, "inbox consume: %s: %v (item left in place)\n", itemPath, err)
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
	stamp.At = time.Now().UTC().Format(time.RFC3339)
	if err := writeConsumedStamp(dest, doc, stamp); err != nil {
		fmt.Fprintf(stderr, "inbox consume: %s consumed, but the consumed stamp write failed: %v\n", dest, err)
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

func parseInboxConsumeArgs(args []string, stderr io.Writer) (string, inboxConsumedStamp, bool) {
	fs := flag.NewFlagSet("inbox consume", flag.ContinueOnError)
	fs.SetOutput(stderr)
	resolution := fs.String("resolution", "", "how the item was resolved (recorded as consumed.resolution)")
	cycle := fs.String("cycle", "console", "cycle that resolved the item (recorded as consumed.cycle)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, inboxConsumeUsage)
		return "", inboxConsumedStamp{}, false
	}
	rest := fs.Args()
	if len(rest) < 1 || rest[0] == "" {
		fmt.Fprintln(stderr, inboxConsumeUsage)
		return "", inboxConsumedStamp{}, false
	}
	itemPath := rest[0]
	if err := fs.Parse(rest[1:]); err != nil || fs.NArg() > 0 {
		fmt.Fprintln(stderr, inboxConsumeUsage)
		return "", inboxConsumedStamp{}, false
	}
	c := strings.TrimSpace(*cycle)
	if n, err := strconv.Atoi(c); c != "console" && (err != nil || n < 0) {
		fmt.Fprintf(stderr, "inbox consume: cycle %q is not a cycle number or \"console\"\n%s\n", *cycle, inboxConsumeUsage)
		return "", inboxConsumedStamp{}, false
	}
	return itemPath, inboxConsumedStamp{Via: "console-manual", Cycle: c, Resolution: strings.TrimSpace(*resolution)}, true
}

func readInboxItemObject(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	if doc == nil {
		return nil, errors.New("not a JSON object: null")
	}
	return doc, nil
}

func writeConsumedStamp(path string, doc map[string]json.RawMessage, stamp inboxConsumedStamp) error {
	b, err := json.Marshal(stamp)
	if err != nil {
		return err
	}
	stamped := make(map[string]json.RawMessage, len(doc)+1)
	for k, v := range doc {
		stamped[k] = v
	}
	stamped["consumed"] = b
	return atomicwrite.JSON(path, stamped)
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
