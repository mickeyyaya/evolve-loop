package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxcurator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	curatorCursorFile = "inbox-curator.json"
	curatorSchema     = "evolve.inbox.curate/1"
)

type curateRequest struct {
	asJSON, apply, event, follow bool
	root                         string
	since                        reader.Since
}

type curatedProposal struct {
	inboxcurator.Proposal
	Applied bool `json:"applied"`
}

type curateReport struct {
	Schema    string            `json:"schema"`
	Stage     string            `json:"stage"`
	Proposals []curatedProposal `json:"proposals"`
}

type curatorRun struct {
	req                       curateRequest
	cfg                       policy.InboxCuratorConfig
	git                       gitexec.Git
	stderr                    io.Writer
	signals                   *signalcenter.Center
	rangeSkipped, applyFailed bool
	landed                    []string
}

func runInboxCurate(root string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, ok := parseInboxCurateArgs(append([]string{"--project-root", root}, args...), stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("curate"))
		return 10
	}
	if req.apply && refusedMidWaveAt(req.root, "curate --apply", stderr) {
		return 1
	}
	pol, err := policy.Load(filepath.Join(req.root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "inbox curate: %v\n", err)
		return 2
	}
	signals, closeSignals := newRoleSignalCenter("inbox", req.root, filepath.Join(req.root, ".evolve"), stderr)
	defer closeSignals()
	run := &curatorRun{req: req, cfg: pol.InboxCuratorConfig(), git: gitexec.Default(req.root), stderr: stderr, signals: signals}
	if req.follow {
		return run.follow(os.Stdout, stdout)
	}
	return run.once(stdin, stdout)
}

func (r *curatorRun) once(stdin io.Reader, stdout io.Writer) int {
	changes, err := r.changes(stdin)
	if err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: %v\n", err)
		return 2
	}
	report, err := r.curate(changes, !r.req.event)
	if err == nil && r.req.apply {
		err = r.land()
	}
	if err != nil {
		fmt.Fprintf(r.stderr, "inbox curate: %v\n", err)
		return 2
	}
	return printCurateReport(report, r.req.asJSON, stdout, r.stderr)
}

func parseInboxCurateArgs(args []string, stderr io.Writer) (curateRequest, bool) {
	var req curateRequest
	var since string
	fs := flag.NewFlagSet("inbox curate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&req.asJSON, "json", false, "print the proposals as JSON")
	fs.BoolVar(&req.apply, "apply", false, "publish the proposals and, in the enforce stage, apply the deterministic ones")
	fs.BoolVar(&req.event, "event", false, "read one ship.landed wire record from stdin")
	fs.BoolVar(&req.follow, "follow", false, "follow the ship channel and curate on each ship.landed event")
	fs.StringVar(&since, "since", "new", "with --follow: new, all, last or an RFC 3339 time")
	fs.StringVar(&req.root, "project-root", "", "the project root")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "inbox curate: %v\n", err)
		return req, false
	}
	if req.root == "" {
		req.root = envOrCwd("EVOLVE_PROJECT_ROOT")
	}
	modes := 0
	for _, on := range []bool{req.apply, req.event, req.follow} {
		if on {
			modes++
		}
	}
	parsed, err := reader.ParseSince(since)
	switch {
	case modes > 1:
		fmt.Fprintln(stderr, "inbox curate: name one of --apply, --event and --follow; only the boundary step applies")
		return req, false
	case err != nil:
		fmt.Fprintf(stderr, "inbox curate: --since: %v\n", err)
		return req, false
	}
	req.since = parsed
	return req, fs.NArg() == 0
}

func (r *curatorRun) changes(stdin io.Reader) ([]inboxcurator.Change, error) {
	if !r.req.event {
		return r.sinceCursor()
	}
	var rec channel.Record
	if err := json.NewDecoder(stdin).Decode(&rec); err != nil {
		return nil, fmt.Errorf("read the wire record from stdin: %w", err)
	}
	return r.landedChanges(rec.Signal)
}

func (r *curatorRun) landedChanges(e *signalcenter.Event) ([]inboxcurator.Change, error) {
	if e == nil || e.Kind != signalcenter.KindShipLanded || e.Fields["commit"] == "" {
		return nil, nil
	}
	return inboxcurator.Commit(r.ctx(), r.git, e.Fields["commit"])
}

func (r *curatorRun) curate(changes []inboxcurator.Change, backstop bool) (curateReport, error) {
	open, warnings, err := inboxcurator.OpenItems(filepath.Join(r.req.root, ".evolve", "inbox"))
	if err != nil {
		return curateReport{}, err
	}
	for _, w := range warnings {
		fmt.Fprintf(r.stderr, "inbox curate: WARN %s\n", w.String())
	}
	tests, err := inboxcurator.IndexTests(filepath.Join(r.req.root, "go"))
	if err != nil {
		return curateReport{}, err
	}
	in := inboxcurator.Inputs{
		Open: open, Changes: changes, IsProtected: laneForbidden(r.req.root, r.stderr), Backstop: backstop,
		RerouteBuildableKinds: r.cfg.RerouteBuildableKinds,
		HasOpenPR:             openPRMatcher(r.ctx(), r.req.root, r.stderr),
		HasTest:               func(name string) bool { return tests[name] || definedByAChange(changes, name) },
		PathExists:            pathExistsUnder(r.req.root),
		Thresholds:            inboxcurator.Thresholds{Refuse: r.cfg.OverlapRefuse, Warn: r.cfg.OverlapWarn}, MinShare: r.cfg.ConsumeMinShare,
	}
	return r.settle(inboxcurator.Curate(in), in.IsProtected), nil
}

type openPRSummary struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	Body        string `json:"body"`
}

func openPRMatcher(ctx context.Context, root string, stderr io.Writer) func(string) bool {
	return openPRMatcherWith(ctx, sysexec.DefaultRunner, root, stderr)
}

func openPRMatcherWith(ctx context.Context, run sysexec.RunFunc, root string, stderr io.Writer) func(string) bool {
	out, err := sysexec.Output(ctx, run, root, "gh", "pr", "list", "--state", "open", "--limit", "1000", "--json", "number,title,headRefName,body")
	if err != nil {
		fmt.Fprintf(stderr, "inbox curate: WARN open-pr lookup failed: %v\n", err)
		return func(string) bool { return false }
	}
	var prs []openPRSummary
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		fmt.Fprintf(stderr, "inbox curate: WARN open-pr decode failed: %v\n", err)
		return func(string) bool { return false }
	}
	return func(itemID string) bool {
		if strings.TrimSpace(itemID) == "" {
			return false
		}
		for _, pr := range prs {
			if containsExactToken(pr.Title, itemID) || containsExactToken(pr.HeadRefName, itemID) || containsExactToken(pr.Body, itemID) {
				return true
			}
		}
		return false
	}
}

func containsExactToken(text, token string) bool {
	for start := 0; ; {
		idx := strings.Index(text[start:], token)
		if idx < 0 {
			return false
		}
		pos := start + idx
		end := pos + len(token)
		beforeOK := pos == 0 || !isIDTokenByte(text[pos-1])
		afterOK := end == len(text) || !isIDTokenByte(text[end])
		if beforeOK && afterOK {
			return true
		}
		start = pos + 1
	}
}

func isIDTokenByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_'
}

func pathExistsUnder(root string) func(string) bool {
	return func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}
}

func definedByAChange(changes []inboxcurator.Change, name string) bool {
	return slices.ContainsFunc(changes, func(c inboxcurator.Change) bool { return slices.Contains(c.Tests, name) })
}

func printCurateReport(report curateReport, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		return encodeInboxJSON("curate", report, stdout, stderr)
	}
	applied, deterministic := 0, 0
	for _, p := range report.Proposals {
		if p.Deterministic {
			deterministic++
		}
		if p.Applied {
			applied++
		}
		fmt.Fprintln(stdout, proposalLine(p))
	}
	fmt.Fprintf(stdout, "inbox curate: stage %s: %d proposals, %d deterministic, %d applied\n", report.Stage, len(report.Proposals), deterministic, applied)
	return 0
}

func proposalLine(p curatedProposal) string {
	target := ""
	if p.Target != "" {
		target = " -> " + p.Target
	}
	state := "judgment"
	switch {
	case p.Applied:
		state = "applied"
	case p.Deterministic:
		state = "deterministic"
	}
	return fmt.Sprintf("%s %s%s (%s, %s): %s; %s", p.Kind, p.ID, target, p.Trigger, state, p.Reason, strings.Join(p.Evidence, "; "))
}
