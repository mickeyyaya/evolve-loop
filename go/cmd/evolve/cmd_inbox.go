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

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func runInbox(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "batches" {
		fmt.Fprintln(stderr, "usage: evolve inbox batches [--json] [--max N]")
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
	batches := inboxbatch.Classify(items, cfg)
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", " ")
		if err := enc.Encode(batches); err != nil {
			fmt.Fprintf(stderr, "inbox batches: encode: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%d items -> %d batches\n", len(items), len(batches))
	fmt.Fprint(stdout, inboxbatch.RenderMarkdown(batches))
	return 0
}
