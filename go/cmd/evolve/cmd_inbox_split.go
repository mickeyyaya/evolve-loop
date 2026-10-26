package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxsplit"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runInboxSplit(root string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inbox split", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print the three ids as one JSON object")
	afterConsole := fs.Bool("lane-after-console", false, "the lane part waits for the console part (deps)")
	projectRoot := &root
	id, ok := parseOnePositional(fs, args)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("split"))
		return 10
	}
	if refusedMidWaveAt(*projectRoot, "split", stderr) {
		return 1
	}
	pol, err := policy.Load(filepath.Join(*projectRoot, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox split: %v\n", err)
		return 2
	}
	opts := inboxmover.Options{ProjectRoot: *projectRoot, Stderr: stderr, IsProtectedPath: laneForbidden(*projectRoot, stderr),
		PriorityClasses: pol.InboxPriorityConfig().ClassOrder}
	res, err := inboxsplit.Accept(newSplitInbox(opts), id, splitForbidden(opts.IsProtectedPath), *afterConsole)
	if err != nil {
		fmt.Fprintf(stderr, "inbox split: %v\n", err)
		return splitExitCode(err)
	}
	if *asJSON {
		body, _ := json.Marshal(map[string]string{"original": res.OriginalID, "console": res.ConsoleID, "lane": res.LaneID})
		fmt.Fprintln(stdout, string(body))
		return 0
	}
	lane := "is lane-dispatchable"
	if res.LaneWaits {
		lane = "waits for " + res.ConsoleID
	}
	fmt.Fprintf(stdout, "inbox split: %s retired to inbox/consumed/; console part %s keeps every protected file, lane part %s %s\n",
		res.OriginalID, res.ConsoleID, res.LaneID, lane)
	return 0
}

func splitForbidden(lane func(string) bool) func(string) bool {
	return func(path string) bool { return lane(path) || guards.TouchesProtectedSurface(path) }
}

func splitExitCode(err error) int {
	for _, refusal := range []error{inboxsplit.ErrNotPending, inboxsplit.ErrHasDependents, inboxsplit.ErrNoFiles,
		inboxsplit.ErrAllProtected, inboxsplit.ErrNoneProtected, inboxsplit.ErrFixNamesProtected, inboxsplit.ErrNoCriterion, inboxsplit.ErrStamped, inboxsplit.ErrMalformed,
		inboxmover.ErrInvalidItem, inboxmover.ErrConsoleRouted} {
		if errors.Is(err, refusal) {
			return 1
		}
	}
	return 2
}

var readSplitItem = readInboxItemObject

type splitInbox struct {
	opts inboxmover.Options
	dir  string
}

func newSplitInbox(opts inboxmover.Options) splitInbox {
	return splitInbox{opts: opts, dir: filepath.Join(opts.ProjectRoot, ".evolve", "inbox")}
}

func (s splitInbox) Item(id string) (inboxsplit.Item, error) {
	path, err := inboxmover.FindFileByTaskID(s.dir, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", inboxsplit.ErrNotPending, id)
	}
	return readSplitItem(path)
}

func (s splitInbox) Dependents(id string) ([]string, error) {
	scan, err := inboxbatch.ScanDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, it := range scan.Items {
		for _, dep := range it.Deps {
			if dep == id {
				out = append(out, it.ID)
			}
		}
	}
	return out, nil
}

func (s splitInbox) File(raw []byte) error {
	_, err := inboxmover.File(s.opts, raw)
	return err
}

func (s splitInbox) RouteLane(id, reason string) error {
	_, err := inboxmover.RouteLane(s.opts, id, reason)
	return err
}

func (s splitInbox) Withdraw(id, reason string) error {
	_, err := inboxmover.Withdraw(s.opts, id, reason)
	return err
}

func (s splitInbox) Retire(id, resolution string) error {
	src, err := inboxmover.FindFileByTaskID(s.dir, id)
	if err != nil {
		return err
	}
	doc, err := readSplitItem(src)
	if err != nil {
		return err
	}
	consumedDir := filepath.Join(s.dir, "consumed")
	if err := os.MkdirAll(consumedDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(consumedDir, filepath.Base(src))
	if err := os.Rename(src, dest); err != nil {
		return err
	}
	stamp := inboxConsumedStamp{At: time.Now().UTC().Format(time.RFC3339), Via: "console-manual", Cycle: "console", Resolution: strings.TrimSpace(resolution)}
	if err := writeConsumedStamp(dest, doc, stamp); err != nil {
		return err
	}
	releaseConsumedItemBinding(s.opts.ProjectRoot, dest, s.opts.Stderr)
	return nil
}
