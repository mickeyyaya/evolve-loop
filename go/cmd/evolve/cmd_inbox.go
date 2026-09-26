// cmd_inbox.go routes `evolve inbox batches` to the deterministic backlog
// classifier (internal/inboxbatch) — the operator view of the SAME grouping
// the triage prompt receives, so "why did triage batch these?" is answerable
// from the terminal without reading a prompt transcript.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

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
	if len(args) < 1 || args[0] != "batches" {
		fmt.Fprintln(stderr, "usage: evolve inbox <batches|quarantine|ack-fingerprint|consume> ...")
		return 10
	}
	asJSON := false
	cfg := inboxbatch.Config{}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--json":
			asJSON = true
		case "--max":
			if i+1 >= len(rest) {
				fmt.Fprintln(stderr, "inbox batches: --max needs a value")
				return 10
			}
			i++
			n, err := strconv.Atoi(rest[i])
			if err != nil {
				fmt.Fprintf(stderr, "inbox batches: bad --max %q\n", rest[i])
				return 10
			}
			cfg.MaxItems = n
		default:
			fmt.Fprintf(stderr, "inbox batches: unknown arg %q\n", rest[i])
			return 10
		}
	}

	// Root resolution matches the inbox-mover sibling: EVOLVE_PROJECT_ROOT
	// wins, else CWD — so running from inside a build worktree still reads
	// the intended project's inbox instead of silently reporting 0 items.
	items, warns, err := inboxbatch.LoadDir(filepath.Join(envOrCwd("EVOLVE_PROJECT_ROOT"), ".evolve", "inbox"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox batches: %v\n", err)
		return 1
	}
	for _, w := range warns {
		fmt.Fprintf(stderr, "inbox batches: WARN skipped %s\n", w)
	}
	// ADR-0074 I1: the operator's own worklist uses the SAME partition triage
	// (internal/phases/triage/triage.go:236) and the claim floor
	// (inboxmover.Claim) already use — console-routed work is operator-owned
	// and is never a batch a lane may draw. Classifying the whole backlog
	// presented it as selectable, with no reason and no separation.
	dispatchable, console, reasons := inboxbatch.PartitionConsole(items, laneForbidden(envOrCwd("EVOLVE_PROJECT_ROOT"), stderr))
	batches := inboxbatch.Classify(dispatchable, cfg)
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", " ")
		doc := inboxBatchesDoc{Batches: batches, ConsoleRouted: consoleRoutedItems(console, reasons)}
		if err := enc.Encode(doc); err != nil {
			fmt.Fprintf(stderr, "inbox batches: encode: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%d items -> %d batches\n", len(items), len(batches))
	fmt.Fprint(stdout, inboxbatch.RenderMarkdown(batches))
	// Loud exclusion: a silently narrowed backlog reads as full coverage.
	if len(console) > 0 {
		fmt.Fprintf(stdout, "%d operator-owned item(s) NOT selectable by a lane (the claim floor refuses them):\n", len(console))
		for _, r := range reasons {
			fmt.Fprintf(stdout, "  * %s\n", r)
		}
	}
	return 0
}

// inboxBatchesDoc is the --json document. Console-routed work is a separate
// bucket rather than a batch, so a machine consumer reads the same routing
// decision the text worklist prints (wiring the partition into one renderer
// only would leave every machine consumer treating operator-owned work as
// dispatchable).
type inboxBatchesDoc struct {
	Batches       []inboxbatch.Batch  `json:"batches"`
	ConsoleRouted []consoleRoutedItem `json:"console_routed,omitempty"`
}

// consoleRoutedItem pairs an operator-owned item with PartitionConsole's own
// reason for routing it.
type consoleRoutedItem struct {
	Item   inboxbatch.Item `json:"item"`
	Reason string          `json:"reason"`
}

// consoleRoutedItems zips PartitionConsole's index-aligned items and reasons,
// dropping the "<id>: " prefix the reason string already carries so the id is
// not stated twice in one record.
func consoleRoutedItems(console []inboxbatch.Item, reasons []string) []consoleRoutedItem {
	out := make([]consoleRoutedItem, len(console))
	for i, it := range console {
		out[i] = consoleRoutedItem{Item: it, Reason: strings.TrimPrefix(reasons[i], it.ID+": ")}
	}
	return out
}
