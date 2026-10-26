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

	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const exitOverlap = 3

type inboxAddRequest struct {
	file, origin, reason    string
	originSet, allowOverlap bool
}

func runInboxAdd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, ok := parseInboxAddArgs(args, stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("add"))
		return 10
	}
	raw, rc := readInboxAddItem(req.file, stdin, stderr)
	if rc != 0 {
		return rc
	}
	root := envOrCwd("EVOLVE_PROJECT_ROOT")
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return 2
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: stderr, IsProtectedPath: laneForbidden(root, stderr),
		PriorityClasses: pol.InboxPriorityConfig().ClassOrder, Origin: req.origin}
	if rc := admitInboxItem(raw, req, pol.InboxCuratorConfig(), &opts, stderr); rc != 0 {
		return rc
	}
	res, err := inboxmover.File(opts, raw)
	switch {
	case errors.Is(err, inboxmover.ErrInvalidItem):
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return 1
	case err != nil:
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "inbox add: filed %s (%s)\n", filepath.Base(res.Path), dispatchability(res.ConsoleReason))
	printFiledRank(res.Path, stdout, stderr)
	warnMonotonicBinaryTargets(raw, stderr)
	return 0
}

func warnMonotonicBinaryTargets(raw []byte, stderr io.Writer) {
	var item inboxbatch.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		fmt.Fprintf(stderr, "inbox add: WARN: acceptance lint skipped: %v\n", err)
		return
	}
	for _, finding := range evalgate.LintMonotonicBinaryTarget(item.Class, item.Acceptance) {
		fmt.Fprintf(stderr, "inbox add: WARN: %s\n", finding)
	}
}

func dispatchability(consoleReason string) string {
	if consoleReason != "" {
		return "console-owned: " + consoleReason
	}
	return "lane-dispatchable"
}

func parseInboxAddArgs(args []string, stderr io.Writer) (inboxAddRequest, bool) {
	req := inboxAddRequest{origin: inboxbatch.OriginConsole}
	fs := flag.NewFlagSet("inbox add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&req.file, "file", "", "read the item JSON from this file instead of stdin")
	fs.StringVar(&req.origin, "origin", req.origin, strings.Join(inboxbatch.Origins(), " | "))
	fs.BoolVar(&req.allowOverlap, "allow-overlap", false, "file an item that overlaps an open item (needs --reason)")
	fs.StringVar(&req.reason, "reason", "", "why an override is right; recorded in the item's intake_override")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return req, false
	}
	if fs.NArg() > 0 {
		return req, false
	}
	fs.Visit(func(f *flag.Flag) { req.originSet = req.originSet || f.Name == "origin" })
	req.reason = strings.TrimSpace(req.reason)
	if req.allowOverlap && req.reason == "" {
		fmt.Fprintln(stderr, "inbox add: --allow-overlap needs --reason")
		return req, false
	}
	return req, true
}

func readInboxAddItem(file string, stdin io.Reader, stderr io.Writer) ([]byte, int) {
	source := stdin
	if file == "" && stdin == nil {
		fmt.Fprintln(stderr, inboxUsage("add"))
		return nil, 10
	}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			fmt.Fprintf(stderr, "inbox add: %v\n", err)
			return nil, 2
		}
		defer func() { _ = f.Close() }()
		source = f
	}
	raw, err := io.ReadAll(source)
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: read the item: %v\n", err)
		return nil, 2
	}
	return raw, 0
}

func admitInboxItem(raw []byte, req inboxAddRequest, cfg policy.InboxCuratorConfig, opts *inboxmover.Options, stderr io.Writer) int {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return 0
	}
	var item inboxbatch.Item
	if err := json.Unmarshal(raw, &item); err != nil {
		fmt.Fprintf(stderr, "inbox add: the item does not decode: %v\n", err)
		return 1
	}
	if req.originSet && item.Origin != "" && item.Origin != req.origin {
		fmt.Fprintf(stderr, "inbox add: the item names origin %q and --origin names %q; name one\n", item.Origin, req.origin)
		return 10
	}
	if item.Origin == "" {
		item.Origin = req.origin
	}
	open, warnings, err := inboxcurator.OpenItems(filepath.Join(opts.ProjectRoot, ".evolve", "inbox"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: the overlap check: %v\n", err)
		return 2
	}
	for _, w := range warnings {
		if !w.Unreadable {
			continue
		}
		fmt.Fprintf(stderr, "inbox add: WARN the overlap check skips a file: %s\n", w.String())
	}
	rules := inboxcurator.Rules{Kinds: cfg.Kinds, IsProtected: opts.IsProtectedPath,
		Thresholds: inboxcurator.Thresholds{Refuse: cfg.OverlapRefuse, Warn: cfg.OverlapWarn}}
	adm, err := inboxcurator.Admit(item, open, rules, inboxcurator.Override{Reason: req.reason, AllowOverlap: req.allowOverlap})
	reportOverlaps(item.ID, adm.Warned, "WARN ", stderr)
	if rc := intakeExitCode(item.ID, err, stderr); rc != 0 {
		return rc
	}
	if adm.Record != nil {
		opts.IntakeRecord, _ = json.Marshal(adm.Record)
	}
	return 0
}

func intakeExitCode(id string, err error, stderr io.Writer) int {
	var overlap *inboxcurator.OverlapError
	switch {
	case errors.As(err, &overlap):
		reportOverlaps(id, overlap.Refused, "", stderr)
		fmt.Fprintf(stderr, "inbox add: refused: extend %s instead, or file with --allow-overlap --reason <why both are needed>\n", strings.Join(overlap.IDs(), ", "))
		return exitOverlap
	case err != nil:
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return 1
	}
	return 0
}

func reportOverlaps(id string, overlaps []inboxcurator.Overlap, prefix string, stderr io.Writer) {
	for _, o := range overlaps {
		fmt.Fprintf(stderr, "inbox add: %s%s overlaps open item %s (similarity %.2f on %s)\n", prefix, id, o.ID, o.Similarity, strings.Join(o.Surface, ", "))
	}
}

var filedRankSource = loadRankedInbox

func printFiledRank(path string, stdout, stderr io.Writer) {
	ranked, err := filedRankSource(time.Now(), io.Discard)
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: WARN the rank is not known: %v\n", err)
		return
	}
	want := ".evolve/inbox/" + filepath.Base(path)
	for _, list := range ranked.doc.Lists {
		for _, row := range list.Items {
			if row.Path == want {
				fmt.Fprintf(stdout, "inbox add: rank %d of %d in the %s list (score %.4f)\n", row.Rank, len(list.Items), list.List, row.Score)
				return
			}
		}
	}
	fmt.Fprintf(stderr, "inbox add: WARN the rank holds no row for %s\n", want)
}
