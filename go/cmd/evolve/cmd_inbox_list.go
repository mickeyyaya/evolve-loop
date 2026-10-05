package main

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type inboxListFilter struct {
	status, kind, route string
}

func (f inboxListFilter) admits(e listedItem) bool {
	return (f.status == "" || e.Status == f.status) && (f.kind == "" || e.Item.Kind == f.kind) && (f.route == "" || e.Item.Route == f.route)
}

type listedItem struct {
	Item   inboxbatch.Item `json:"item"`
	Path   string          `json:"path"`
	Status string          `json:"status"`
	Reason string          `json:"reason,omitempty"`
}

func runInboxList(args []string, stdout, stderr io.Writer) int {
	filter, asJSON, ok := parseInboxListArgs(args, stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("list"))
		return 10
	}
	inbox, err := loadPendingInbox("list", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "inbox list: %v\n", err)
		return 2
	}
	listed := []listedItem{}
	for _, it := range inbox.items {
		place, reason := inbox.place(it)
		if e := (listedItem{Item: it, Path: ".evolve/inbox/" + it.Path, Status: menuStatusNames[place], Reason: reason}); filter.admits(e) {
			listed = append(listed, e)
		}
	}
	if asJSON {
		return encodeInboxJSON("list", listed, stdout, stderr)
	}
	fmt.Fprintf(stdout, "%d item(s)\n", len(listed))
	for _, e := range listed {
		fmt.Fprintf(stdout, "  %s  %s  [%s]  w=%g  %s\n", e.Item.ID, statusLine(e.Status, e.Reason), e.Item.Kind, e.Item.Weight, e.Item.Title)
	}
	return 0
}

func parseInboxListArgs(args []string, stderr io.Writer) (inboxListFilter, bool, bool) {
	var filter inboxListFilter
	fs := flag.NewFlagSet("inbox list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&filter.status, "status", "", strings.Join(menuStatusNames[:], " | "))
	fs.StringVar(&filter.kind, "kind", "", "only items of this kind")
	fs.StringVar(&filter.route, "route", "", "only items carrying this route")
	asJSON := fs.Bool("json", false, "print a JSON array")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return filter, false, false
	}
	if filter.status == "" || slices.Contains(menuStatusNames[:], filter.status) {
		return filter, *asJSON, true
	}
	fmt.Fprintf(stderr, "inbox list: --status %q is not one of %s\n", filter.status, strings.Join(menuStatusNames[:], ", "))
	return filter, false, false
}
