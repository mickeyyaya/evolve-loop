package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
)

type inboxVerb struct {
	name, synopsis string
	run            func(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

func inboxVerbs() []inboxVerb {
	withoutStdin := func(run func(args []string, stdout, stderr io.Writer) int) func([]string, io.Reader, io.Writer, io.Writer) int {
		return func(args []string, _ io.Reader, stdout, stderr io.Writer) int { return run(args, stdout, stderr) }
	}
	return []inboxVerb{
		{"batches", "batches [--json] [--max N]", withoutStdin(runInboxBatches)},
		{"list", "list [--status " + strings.Join(menuStatusNames[:], "|") + "] [--kind K] [--route R] [--json]", withoutStdin(runInboxList)},
		{"show", "show <id> [--json]", withoutStdin(runInboxShow)},
		{"rank", "rank [--list " + strings.Join(rankListNames, "|") + "] [--top N] [--explain <id>] [--json]", withoutStdin(runInboxRank)},
		{"add", "add [--file <item.json>]   (without --file, the item JSON is read from stdin)", runInboxAdd},
		{"edit", "edit <id|item-path> (--set F=V | --add F=V | --remove F=V)...   (--set on a list field takes a JSON array)", withoutStdin(runInboxEdit)},
		{"verify", "verify <id> --evidence <text>", withoutStdin(runInboxVerify)},
		{"withdraw", "withdraw <id> <reason>", withoutStdin(runInboxWithdraw)},
		{"claims", "claims [--json] [--project-root P]", withoutStdin(runInboxClaims)},
		{"release", "release <id> <reason> [--json] | release --stale <reason> [--json] [--project-root P]", withoutStdin(runInboxRelease)},
		{"route-console", "route-console <id> <reason> <cycle>", withoutStdin(runInboxRouteConsole)},
		{"route-lane", "route-lane <id> <reason>", withoutStdin(runInboxRouteLane)},
		{"consume", "consume <item-path> [--resolution <text>] [--cycle <n>|console]", withoutStdin(runInboxConsume)},
		{"quarantine", "quarantine list [--json] | quarantine release <id>", runInboxQuarantine},
		{"ack-fingerprint", "ack-fingerprint <item-path>", withoutStdin(runInboxAckFingerprint)},
	}
}

func runInbox(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	verbs := inboxVerbs()
	if len(args) == 0 {
		printInboxUsage(stderr, verbs)
		return 10
	}
	if args[0] == "--help" || args[0] == "-h" {
		printInboxUsage(stdout, verbs)
		return 0
	}
	for _, v := range verbs {
		if args[0] == v.name {
			return v.run(args[1:], stdin, stdout, stderr)
		}
	}
	printInboxUsage(stderr, verbs)
	return 10
}

func inboxUsage(verb string) string {
	for _, v := range inboxVerbs() {
		if v.name == verb {
			return "usage: evolve inbox " + v.synopsis
		}
	}
	return "usage: evolve inbox <verb> ..."
}

func printInboxUsage(w io.Writer, verbs []inboxVerb) {
	fmt.Fprintln(w, "usage: evolve inbox <verb> ...")
	for _, v := range verbs {
		fmt.Fprintf(w, "  evolve inbox %s\n", v.synopsis)
	}
}

type pendingInbox struct {
	items       []inboxbatch.Item
	opts        inboxmover.Options
	isProtected func(string) bool
}

func loadPendingInbox(verb string, stderr io.Writer) (pendingInbox, error) {
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	items, warns, err := inboxbatch.LoadDir(inboxDir)
	if err != nil {
		return pendingInbox{}, err
	}
	for _, w := range warns {
		fmt.Fprintf(stderr, "inbox %s: WARN skipped %s\n", verb, w)
	}
	opts := inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}
	return pendingInbox{items: items, opts: opts, isProtected: laneForbidden(root, stderr)}, nil
}

func (p pendingInbox) place(it inboxbatch.Item) (inboxmover.MenuPlace, string) {
	return inboxmover.PlaceOnLaneMenu(p.opts, it, p.isProtected)
}

func runInboxBatches(args []string, stdout, stderr io.Writer) int {
	asJSON, cfg, ok := parseInboxBatchesArgs(args, stderr)
	if !ok {
		return 10
	}
	inbox, err := loadPendingInbox("batches", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "inbox batches: %v\n", err)
		return 1
	}
	rank := loadRankInputs("batches", filepath.Dir(inbox.opts.InboxDir), time.Now(), stderr)
	menu := inboxmover.RankLaneMenu(inbox.opts, inbox.items, inbox.isProtected, rank)
	cfg.Order = inboxrank.Sequence(menu.Ranked)
	batches := inboxbatch.Classify(menu.Ready, cfg)
	if asJSON {
		return encodeInboxBatches(stdout, stderr, inboxBatchesDoc{Batches: batches, ConsoleRouted: excludedItems(menu.Console, menu.ConsoleReasons), DependencyBlocked: excludedItems(menu.Waiting, menu.WaitingReasons)})
	}
	fmt.Fprintf(stdout, "%d items -> %d batches\n", len(inbox.items), len(batches))
	fmt.Fprint(stdout, inboxbatch.RenderMarkdown(batches, inboxrank.Labels(menu.Ranked)))
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
