package inboxcmd

import (
	"context"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const exitOverlap = exitRefused

type inboxAddRequest struct {
	file, origin, reason            string
	originSet, allowOverlap, asJSON bool
}

func runInboxAdd(root string, args []string, stdin io.Reader, stdout, stderr io.Writer, deps Deps) int {
	req, ok := parseInboxAddArgs(args, stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("add"))
		return 10
	}
	raw, rc := readInboxAddItem(req.file, stdin, stderr)
	if rc != 0 {
		return rc
	}
	pol, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return 2
	}
	signals, closeSignals := deps.newSignalCenter("inbox", root, filepath.Join(root, ".evolve"), stderr)
	defer closeSignals()
	res, err := intakeItem(context.Background(), intakeInput{root: root, raw: raw, req: req, pol: pol,
		isProtected: deps.laneForbidden(root, stderr), signals: signals, stderr: stderr})
	if err != nil {
		return intakeFailure(err, stderr)
	}
	printFiledRank(root, res.Path, deps.humanWriter(stdout, req.asJSON), stderr, deps)
	warnMonotonicBinaryTargets(raw, stderr)
	result := inboxWriteResult{Verb: "add", Path: res.Path, Detail: dispatchability(res.ConsoleReason)}
	return finishInboxWrite(deps, req.asJSON, result, fmt.Sprintf("inbox add: filed %s (%s)", filepath.Base(res.Path), result.Detail), stdout, stderr)
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
	fs.StringVar(&req.file, "file", "", "read the item JSON from this file; - reads stdin")
	fs.BoolVar(&req.asJSON, "json", false, "print the result as JSON")
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
	if file == "" || (file == "-" && stdin == nil) {
		fmt.Fprintln(stderr, inboxUsage("add"))
		return nil, 10
	}
	if file != "-" {
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

func probeTree(root string, item inboxbatch.Item) (inboxcurator.Tree, error) {
	tree := inboxcurator.Tree{PathExists: pathExistsUnder(root)}
	if len(item.Probes) == 0 {
		return tree, nil
	}
	tests, err := inboxcurator.IndexTests(filepath.Join(root, "go"))
	tree.HasTest = func(name string) bool { return tests[name] }
	return tree, err
}

func intakeFailure(err error, stderr io.Writer) int {
	var refused *RefusedError
	if !errors.As(err, &refused) {
		fmt.Fprintf(stderr, "inbox add: %v\n", err)
		return exitIO
	}
	var overlap *inboxcurator.OverlapError
	switch {
	case errors.As(err, &overlap):
		reportRefusedOverlap(refused, overlap, stderr)
		return exitOverlap
	case errors.Is(err, errOriginConflict):
		fmt.Fprintf(stderr, "inbox add: %s\n", refused.Cause)
		return exitUsage
	}
	fmt.Fprintf(stderr, "inbox add: %s\n", refused.Cause)
	return exitFailed
}

func reportRefusedOverlap(refused *RefusedError, overlap *inboxcurator.OverlapError, stderr io.Writer) {
	for _, line := range strings.Split(refused.Cause, "\n") {
		fmt.Fprintf(stderr, "inbox add: %s\n", line)
	}
	fmt.Fprintf(stderr, "inbox add: refused: extend %s instead, or file with --allow-overlap --reason <why both are needed>\n", strings.Join(overlap.IDs(), ", "))
}

func overlapLines(id string, overlaps []inboxcurator.Overlap, prefix string) []string {
	lines := make([]string, len(overlaps))
	for i, o := range overlaps {
		lines[i] = fmt.Sprintf("%s%s overlaps open item %s (similarity %.2f on %s)", prefix, id, o.ID, o.Similarity, strings.Join(o.Surface, ", "))
	}
	return lines
}

func reportOverlaps(id string, overlaps []inboxcurator.Overlap, prefix string, stderr io.Writer) {
	for _, line := range overlapLines(id, overlaps, prefix) {
		fmt.Fprintf(stderr, "inbox add: %s\n", line)
	}
}

var filedRankSource = loadRankedInbox

func printFiledRank(root, path string, stdout, stderr io.Writer, deps Deps) {
	ranked, err := filedRankSource(root, time.Now(), io.Discard, deps)
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
