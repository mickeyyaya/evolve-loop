package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

type inboxVerb struct {
	name, synopsis string
	run            func(root string, args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

func inboxVerbs() []inboxVerb {
	withoutStdin := func(run func(root string, args []string, stdout, stderr io.Writer) int) func(string, []string, io.Reader, io.Writer, io.Writer) int {
		return func(root string, args []string, _ io.Reader, stdout, stderr io.Writer) int {
			return run(root, args, stdout, stderr)
		}
	}
	return []inboxVerb{
		{"batches", "batches [--json] [--max N]", withoutStdin(runInboxBatches)},
		{"lint", "lint [--json]", withoutStdin(runInboxLint)},
		{"list", "list [--status " + strings.Join(menuStatusNames[:], "|") + "] [--kind K] [--route R] [--json]", withoutStdin(runInboxList)},
		{"show", "show <id> [--json]", withoutStdin(runInboxShow)},
		{"rank", "rank [--list " + strings.Join(rankListNames, "|") + "] [--top N] [--explain <id>] [--json]", withoutStdin(runInboxRank)},
		{"add", "add --file <item.json|-> [--origin O] [--allow-overlap --reason R] [--json]   (--file - reads the item JSON from stdin)", runInboxAdd},
		{"edit", "edit <id|item-path> (--set F=V | --add F=V | --remove F=V)... [--json]   (--set on a list field takes a JSON array)", withoutStdin(runInboxEdit)},
		{"verify", "verify <id> --evidence <text> [--json]", withoutStdin(runInboxVerify)},
		{"withdraw", "withdraw <id> <reason> [--json]", withoutStdin(runInboxWithdraw)},
		{"claims", "claims [--json] [--project-root P]", withoutStdin(runInboxClaims)},
		{"release", "release <id> <reason> [--keep root|claim] [--json] | release --stale <reason> [--json] [--project-root P]", withoutStdin(runInboxRelease)},
		{"split", "split <id> [--lane-after-console] [--json] [--project-root P]", withoutStdin(runInboxSplit)},
		{"route-console", "route-console <id> <reason> <cycle> [--json]", withoutStdin(runInboxRouteConsole)},
		{"route-lane", "route-lane <id> <reason> [--json]", withoutStdin(runInboxRouteLane)},
		{"consume", "consume <item-path> [--resolution <text>] [--cycle <n>|console] [--json]", withoutStdin(runInboxConsume)},
		{"curate", "curate [--json] [--apply | --event] [--project-root P]   (--event reads one ship.landed wire record from stdin)", runInboxCurate},
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
		if args[0] != v.name {
			continue
		}
		root, rest, ok := takeProjectRoot(args[1:])
		if !ok {
			fmt.Fprintln(stderr, inboxUsage(v.name))
			return cliExitUsage
		}
		return v.run(root, rest, stdin, stdout, stderr)
	}
	printInboxUsage(stderr, verbs)
	return 10
}

type inboxWriteResult struct {
	Verb   string `json:"verb"`
	ID     string `json:"id,omitempty"`
	Path   string `json:"path"`
	Detail string `json:"detail,omitempty"`
}

func takeInboxJSON(args []string) (bool, []string) {
	i := slices.Index(args, "--json")
	if i < 0 {
		return false, args
	}
	return true, slices.Concat(args[:i], args[i+1:])
}

func finishInboxWrite(asJSON bool, r inboxWriteResult, human string, stdout, stderr io.Writer) int {
	if asJSON {
		return printVerbJSON("inbox "+r.Verb, r, stdout, stderr)
	}
	fmt.Fprintln(stdout, human)
	return cliExitOK
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
	fmt.Fprintln(w, "usage: evolve inbox <verb> ...   (each verb takes --project-root P)")
	for _, v := range verbs {
		fmt.Fprintf(w, "  evolve inbox %s\n", v.synopsis)
	}
}

type pendingInbox struct {
	items       []inboxbatch.Item
	opts        inboxmover.Options
	isProtected func(string) bool
}

func loadPendingInbox(root, verb string, stderr io.Writer) (pendingInbox, error) {
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	scan, err := inboxbatch.ScanDir(inboxDir)
	if err != nil {
		return pendingInbox{}, err
	}
	printLoadWarnings(stderr, "inbox "+verb, scan.Warnings)
	opts := inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}
	return pendingInbox{items: scan.Items, opts: opts, isProtected: laneForbidden(root, stderr)}, nil
}

func printLoadWarnings(stderr io.Writer, prefix string, warns []inboxbatch.LoadWarning) {
	for _, w := range warns {
		fmt.Fprintf(stderr, "%s: %s\n", prefix, w.String())
	}
}

func (p pendingInbox) place(it inboxbatch.Item) (inboxmover.MenuPlace, string) {
	return inboxmover.PlaceOnLaneMenu(p.opts, it, p.isProtected)
}

func runInboxBatches(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, cfg, ok := parseInboxBatchesArgs(args, stderr)
	if !ok {
		return 10
	}
	inbox, err := loadPendingInbox(root, "batches", stderr)
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

type inboxLintEntry struct {
	ID                 string   `json:"id"`
	ProtectedBasenames []string `json:"protected_basenames"`
}

type inboxLintDoc struct {
	Undeclared []inboxLintEntry `json:"undeclared"`
}

func runInboxLint(root string, args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			continue
		}
		fmt.Fprintf(stderr, "inbox lint: unknown arg %q\n", arg)
		return 10
	}
	inbox, err := loadPendingInbox(root, "lint", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "inbox lint: %v\n", err)
		return 1
	}
	isProtectedBase := protectedBasenameJudge(root, inbox.isProtected, stderr)
	var undeclared []inboxLintEntry
	for _, it := range inbox.items {
		if routed, _ := inboxbatch.ConsoleRouted(it, inbox.isProtected); routed || it.HasDeclaredSurface() {
			continue
		}
		undeclared = append(undeclared, inboxLintEntry{
			ID:                 it.ID,
			ProtectedBasenames: it.ProtectedBasenames(isProtectedBase),
		})
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inboxLintDoc{Undeclared: undeclared}); err != nil {
			fmt.Fprintf(stderr, "inbox lint: encode: %v\n", err)
			return 1
		}
	} else {
		for _, e := range undeclared {
			if len(e.ProtectedBasenames) > 0 {
				fmt.Fprintf(stdout, "%s: undeclared surface (cites protected basename: %s)\n", e.ID, strings.Join(e.ProtectedBasenames, ", "))
			} else {
				fmt.Fprintf(stdout, "%s: undeclared surface\n", e.ID)
			}
		}
	}
	if len(undeclared) > 0 {
		return 1
	}
	return 0
}

func protectedBasenameJudge(root string, isProtected func(string) bool, stderr io.Writer) func(string) bool {
	protected := map[string]bool{}
	if isProtected != nil {
		for _, e := range guards.ProtectedSurfaceManifest {
			rel := strings.TrimPrefix(e.Fragment, "/")
			if !strings.HasSuffix(rel, "/") && isProtected(rel) {
				protected[filepath.Base(rel)] = true
			}
		}
		tracked, err := repostate.TrackedTree(root, ".")
		if err != nil {
			fmt.Fprintf(stderr, "inbox lint: tracked files unreadable, judging basenames by the protected-surface manifest only: %v\n", err)
		}
		for _, rel := range tracked {
			if isProtected(filepath.ToSlash(rel)) {
				protected[filepath.Base(rel)] = true
			}
		}
	}
	return func(name string) bool { return protected[name] }
}
