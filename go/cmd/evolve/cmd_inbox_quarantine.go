// cmd_inbox_quarantine.go — operator surface for ADR-0072 S5 task quarantine.
//
//	evolve inbox quarantine list [--json]     show quarantined poison todos
//	evolve inbox quarantine release <id>      un-quarantine an item (reset its
//	                                          failure count, return to inbox root)
//
// This is the escape hatch for the automatic S5 quarantine wired into the loop's
// cycle-failure drain: an operator inspects why a todo was quarantined (the item
// JSON carries failure_count + last_failure_reason) and, once the root cause is
// fixed, releases it back into triage.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runInboxQuarantine(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: evolve inbox quarantine <list|release> ...")
		return 10
	}
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	qDir := filepath.Join(root, ".evolve", "inbox", "quarantine")

	switch args[0] {
	case "list":
		asJSON := false
		for _, a := range args[1:] {
			if a == "--json" {
				asJSON = true
			} else {
				fmt.Fprintf(stderr, "inbox quarantine list: unknown arg %q\n", a)
				return 10
			}
		}
		items, warns, err := inboxbatch.LoadDir(qDir)
		if err != nil {
			fmt.Fprintf(stderr, "inbox quarantine list: %v\n", err)
			return 1
		}
		for _, w := range warns {
			fmt.Fprintf(stderr, "inbox quarantine list: WARN skipped %s\n", w)
		}
		if asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", " ")
			if err := enc.Encode(items); err != nil {
				fmt.Fprintf(stderr, "inbox quarantine list: encode: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintf(stdout, "%d quarantined item(s)\n", len(items))
		for _, it := range items {
			fmt.Fprintf(stdout, "  %s\t%s\n", it.ID, it.Title)
		}
		return 0

	case "release":
		if len(args) < 2 || args[1] == "" {
			fmt.Fprintln(stderr, "usage: evolve inbox quarantine release <id>")
			return 10
		}
		id := args[1]
		res, err := inboxmover.ReleaseFromQuarantine(inboxmover.Options{ProjectRoot: root, Stderr: stderr}, id)
		if err != nil {
			fmt.Fprintf(stderr, "inbox quarantine release: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "released %s → %s\n", id, res.DestPath)
		return 0

	default:
		fmt.Fprintf(stderr, "inbox quarantine: unknown subcommand %q (want list|release)\n", args[0])
		return 10
	}
}
