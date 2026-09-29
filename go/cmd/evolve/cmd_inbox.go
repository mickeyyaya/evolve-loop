package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// runInbox dispatches `evolve inbox <quarantine|ack-fingerprint|consume|route-console|batches>`.
// The batches path routes to the deterministic backlog classifier
// (internal/inboxbatch) — the operator view of the SAME grouping the triage
// prompt receives, so "why did triage batch these?" is answerable from the
// terminal without reading a prompt transcript.
func runInbox(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) >= 1 && args[0] == "quarantine" {
		return runInboxQuarantine(args[1:], stdin, stdout, stderr)
	}
	if len(args) >= 1 && args[0] == "ack-fingerprint" {
		return runInboxAckFingerprint(args[1:], stdout, stderr)
	}
	if len(args) >= 1 && args[0] == "consume" {
		return runInboxConsume(args[1:], stdout, stderr)
	}
	if len(args) >= 1 && args[0] == "route-console" {
		return runInboxRouteConsole(args[1:], stdout, stderr)
	}
	if len(args) < 1 || args[0] != "batches" {
		fmt.Fprintln(stderr, "usage: evolve inbox <batches|quarantine|ack-fingerprint|consume|route-console> ...")
		return 10
	}
	return runInboxBatches(args[1:], stdout, stderr)
}

func runInboxBatches(args []string, stdout, stderr io.Writer) int {
	asJSON, cfg, ok := parseInboxBatchesArgs(args, stderr)
	if !ok {
		return 10
	}
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	items, warns, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		fmt.Fprintf(stderr, "inbox batches: %v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintf(stderr, "inbox batches: WARN skipped %s\n", w)
	}
	menu := inboxmover.PartitionLaneMenu(inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}, items, laneForbidden(root, stderr))
	batches := inboxbatch.Classify(menu.Ready, cfg)
	if asJSON {
		return encodeInboxBatches(stdout, stderr, inboxBatchesDoc{Batches: batches, ConsoleRouted: excludedItems(menu.Console, menu.ConsoleReasons), DependencyBlocked: excludedItems(menu.Waiting, menu.WaitingReasons)})
	}
	fmt.Fprintf(stdout, "%d items -> %d batches\n", len(items), len(batches))
	fmt.Fprint(stdout, inboxbatch.RenderMarkdown(batches))
	listExcluded(stdout, "%d operator-owned item(s) NOT selectable by a lane (the claim floor refuses them):\n", menu.ConsoleReasons)
	listExcluded(stdout, "%d item(s) waiting on a dependency (not selectable until it lands):\n", menu.WaitingReasons)
	return 0
}

func parseInboxBatchesArgs(args []string, stderr io.Writer) (asJSON bool, cfg inboxbatch.Config, ok bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--max":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "inbox batches: --max needs a value")
				return false, cfg, false
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "inbox batches: bad --max %q\n", args[i])
				return false, cfg, false
			}
			cfg.MaxItems = n
		default:
			fmt.Fprintf(stderr, "inbox batches: unknown arg %q\n", args[i])
			return false, cfg, false
		}
	}
	return asJSON, cfg, true
}

func encodeInboxBatches(stdout, stderr io.Writer, doc inboxBatchesDoc) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(stderr, "inbox batches: encode: %v\n", err)
		return 1
	}
	return 0
}

func listExcluded(stdout io.Writer, heading string, reasons []string) {
	if len(reasons) == 0 {
		return
	}
	fmt.Fprintf(stdout, heading, len(reasons))
	for _, r := range reasons {
		fmt.Fprintf(stdout, "  * %s\n", r)
	}
}

type inboxBatchesDoc struct {
	Batches           []inboxbatch.Batch `json:"batches"`
	ConsoleRouted     []excludedItem     `json:"console_routed,omitempty"`
	DependencyBlocked []excludedItem     `json:"dependency_blocked,omitempty"`
}

type excludedItem struct {
	Item   inboxbatch.Item `json:"item"`
	Reason string          `json:"reason"`
}

func excludedItems(items []inboxbatch.Item, reasons []string) []excludedItem {
	out := make([]excludedItem, len(items))
	for i, it := range items {
		out[i] = excludedItem{Item: it, Reason: strings.TrimPrefix(reasons[i], it.ID+": ")}
	}
	return out
}
