package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const allRankLists = "all"

var rankListNames = append(slices.Clone(menuStatusNames[:]), allRankLists)

type inboxRankRequest struct {
	asJSON  bool
	explain string
	top     int
	list    string
}

type rankedRow struct {
	Rank     int     `json:"rank"`
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Path     string  `json:"path"`
	Weight   float64 `json:"weight"`
	Declared bool    `json:"declared"`
	inboxrank.Breakdown
}

type rankedList struct {
	List  string      `json:"list"`
	Items []rankedRow `json:"items"`
}

type rankedInboxDoc struct {
	AsOf  string       `json:"as_of"`
	Lists []rankedList `json:"lists"`
}

type explainedItemDoc struct {
	List string `json:"list"`
	Of   int    `json:"of"`
	rankedRow
}

type rankedInbox struct {
	doc  rankedInboxDoc
	cfg  policy.InboxPriorityConfig
	opts inboxmover.Options
}

func runInboxRank(args []string, stdout, stderr io.Writer) int {
	return rankInboxAt(args, time.Now(), stdout, stderr)
}

func rankInboxAt(args []string, now time.Time, stdout, stderr io.Writer) int {
	req, ok := parseInboxRankArgs(args, stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("rank"))
		return 10
	}
	ranked, err := loadRankedInbox(now, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "inbox rank: %v\n", err)
		return 2
	}
	if req.explain != "" {
		return explainRanked(ranked, req, stdout, stderr)
	}
	doc := ranked.doc
	doc.Lists = selectRankedLists(doc.Lists, req)
	if req.asJSON {
		return encodeInboxJSON("rank", doc, stdout, stderr)
	}
	printRankedLists(stdout, doc.Lists, rankedCounts(ranked.doc.Lists))
	return 0
}

func parseInboxRankArgs(args []string, stderr io.Writer) (inboxRankRequest, bool) {
	req := inboxRankRequest{list: menuStatusNames[inboxmover.MenuReady]}
	fs := flag.NewFlagSet("inbox rank", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&req.asJSON, "json", false, "print JSON")
	fs.StringVar(&req.explain, "explain", "", "print one item's factor breakdown")
	fs.IntVar(&req.top, "top", 0, "show only the first N items of each list")
	fs.StringVar(&req.list, "list", req.list, strings.Join(rankListNames, " | "))
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return req, false
	}
	named := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { named[f.Name] = true })
	req.explain = strings.TrimSpace(req.explain)
	switch {
	case named["explain"] && req.explain == "":
		fmt.Fprintln(stderr, "inbox rank: --explain needs an item id")
	case named["top"] && req.top < 1:
		fmt.Fprintf(stderr, "inbox rank: --top must be at least 1, got %d\n", req.top)
	case !slices.Contains(rankListNames, req.list):
		fmt.Fprintf(stderr, "inbox rank: --list %q is not one of %s\n", req.list, strings.Join(rankListNames, ", "))
	default:
		return req, true
	}
	return req, false
}

func loadRankedInbox(now time.Time, stderr io.Writer) (rankedInbox, error) {
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	evolveDir := filepath.Join(root, ".evolve")
	if _, err := policy.Load(filepath.Join(evolveDir, "policy.json")); err != nil {
		return rankedInbox{}, err
	}
	inbox, err := loadPendingInbox("rank", stderr)
	if err != nil {
		return rankedInbox{}, err
	}
	in := loadRankInputs("rank", evolveDir, now, stderr)
	for _, warning := range inboxrank.ClassWarnings(inbox.items, in.Config) {
		fmt.Fprintf(stderr, "inbox rank: WARN %s\n", warning)
	}
	menu := inboxmover.PartitionLaneMenu(inbox.opts, inbox.items, inbox.isProtected)
	doc := rankedInboxDoc{AsOf: now.UTC().Format(time.RFC3339)}
	for _, list := range []struct {
		place inboxmover.MenuPlace
		items []inboxbatch.Item
	}{{inboxmover.MenuReady, menu.Ready}, {inboxmover.MenuConsole, menu.Console}, {inboxmover.MenuWaiting, menu.Waiting}} {
		doc.Lists = append(doc.Lists, rankedList{List: menuStatusNames[list.place], Items: rankedRows(in.Order(list.items, inbox.items))})
	}
	return rankedInbox{doc: doc, cfg: in.Config, opts: inbox.opts}, nil
}

func loadRankInputs(verb, evolveDir string, now time.Time, stderr io.Writer) inboxrank.Inputs {
	in, warnings := rankinputs.Load(evolveDir, now)
	for _, w := range warnings {
		fmt.Fprintf(stderr, "inbox %s: WARN %s\n", verb, w)
	}
	return in
}

func rankedRows(ranked []inboxrank.Ranked) []rankedRow {
	rows := make([]rankedRow, len(ranked))
	for i, r := range ranked {
		rows[i] = rankedRow{
			Rank: r.Rank, ID: r.Item.ID, Title: r.Item.Title, Path: ".evolve/inbox/" + r.Item.Path,
			Weight: r.Item.Weight, Declared: r.Item.DeclaredSurface(), Breakdown: r.Breakdown,
		}
	}
	return rows
}

func selectRankedLists(lists []rankedList, req inboxRankRequest) []rankedList {
	var out []rankedList
	for _, l := range lists {
		if req.list != allRankLists && l.List != req.list {
			continue
		}
		if req.top > 0 && len(l.Items) > req.top {
			l.Items = l.Items[:req.top]
		}
		out = append(out, l)
	}
	return out
}
