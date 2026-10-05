package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

var menuStatusNames = [...]string{
	inboxmover.MenuReady:   "ready",
	inboxmover.MenuConsole: "console",
	inboxmover.MenuWaiting: "waiting",
}

type shownItem struct {
	ID     string          `json:"id"`
	Path   string          `json:"path"`
	Status string          `json:"status"`
	Reason string          `json:"reason,omitempty"`
	Item   json.RawMessage `json:"item"`
}

func runInboxShow(args []string, stdout, stderr io.Writer) int {
	id, asJSON, ok := parseInboxShowArgs(args)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("show"))
		return 10
	}
	inbox, err := loadPendingInbox("show", stderr)
	if err != nil {
		fmt.Fprintf(stderr, "inbox show: %v\n", err)
		return 2
	}
	for _, it := range inbox.items {
		if it.ID == id {
			return printShownItem(inbox, it, asJSON, stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "inbox show: %s\n", notPendingReason(inbox.opts, id))
	return 1
}

func parseInboxShowArgs(args []string) (id string, asJSON, ok bool) {
	for _, a := range args {
		switch {
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, "-") || id != "" || strings.TrimSpace(a) == "":
			return "", false, false
		default:
			id = a
		}
	}
	return id, asJSON, id != ""
}

func notPendingReason(opts inboxmover.Options, id string) string {
	ds := inboxmover.ResolveDispatchState(opts, id)
	if ds.State == inboxmover.StateUnknown {
		return "no inbox item " + id
	}
	return strings.TrimSpace(fmt.Sprintf("%s is not pending: it is %s %s", id, ds.State, ds.Detail))
}

func printShownItem(inbox pendingInbox, it inboxbatch.Item, asJSON bool, stdout, stderr io.Writer) int {
	raw, err := os.ReadFile(filepath.Join(inbox.opts.InboxDir, it.Path))
	if err != nil {
		fmt.Fprintf(stderr, "inbox show: %v\n", err)
		return 2
	}
	place, reason := inbox.place(it)
	shown := shownItem{ID: it.ID, Path: ".evolve/inbox/" + it.Path, Status: menuStatusNames[place], Reason: reason, Item: raw}
	if asJSON {
		return encodeInboxJSON("show", shown, stdout, stderr)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		fmt.Fprintf(stderr, "inbox show: %s: %v\n", shown.Path, err)
		return 2
	}
	fmt.Fprintf(stdout, "%s: %s\npath: %s\n%s\n", shown.ID, statusLine(shown.Status, shown.Reason), shown.Path, pretty.String())
	return 0
}

func statusLine(status, reason string) string {
	if reason == "" {
		return status
	}
	return status + " (" + reason + ")"
}

func encodeInboxJSON(verb string, doc any, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(doc); err != nil {
		fmt.Fprintf(stderr, "inbox %s: encode: %v\n", verb, err)
		return 2
	}
	return 0
}
