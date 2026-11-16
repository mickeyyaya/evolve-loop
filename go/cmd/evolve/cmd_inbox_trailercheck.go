package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/closesinbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type trailerCheckReport struct {
	Range      string   `json:"range"`
	Stage      string   `json:"stage"`
	OK         bool     `json:"ok"`
	Closes     []string `json:"closes"`
	Refs       []string `json:"refs,omitempty"`
	Violations []string `json:"violations"`
}

func parseInboxTrailerCheckArgs(args []string, stderr io.Writer) (string, bool, bool) {
	var rng string
	var asJSON bool
	fs := flag.NewFlagSet("inbox trailer-check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&rng, "range", "", "git revision range")
	fs.BoolVar(&asJSON, "json", false, "emit JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || strings.TrimSpace(rng) == "" {
		fmt.Fprintln(stderr, inboxUsage("trailer-check"))
		return "", false, false
	}
	return strings.TrimSpace(rng), asJSON, true
}

func checkInboxTrailerBytes(root string, body []byte, label string) ([]string, []string, []string) {
	closes := closesinbox.IDs(body)
	refs := closesinbox.RefIDs(body)
	if !closesinbox.Declares(body) {
		return closes, refs, []string{"no Closes-Inbox or Refs-Inbox trailer in " + label}
	}
	opts := inboxmover.Options{InboxDir: filepath.Join(root, ".evolve", "inbox"), Stderr: io.Discard}
	var violations []string
	for _, id := range slices.Concat(closes, refs) {
		ds := inboxmover.ResolveDispatchState(opts, id)
		if ds.State != inboxmover.StatePending && ds.State != inboxmover.StateProcessing {
			violations = append(violations, fmt.Sprintf("%s is not an open inbox item (state %s)", id, ds.State))
		}
	}
	return closes, refs, violations
}

func checkInboxTrailerRange(root, rng string) ([]string, []string, []string, error) {
	out, err := sysexec.Command(context.Background(), "git", "-C", root, "log", "--format=%B", rng).CombinedOutput()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git log %s: %w (%s)", rng, err, strings.TrimSpace(string(out)))
	}
	closes, refs, violations := checkInboxTrailerBytes(root, out, rng)
	return closes, refs, violations, nil
}

func runInboxTrailerCheck(root string, args []string, stdout, stderr io.Writer) int {
	rng, asJSON, ok := parseInboxTrailerCheckArgs(args, stderr)
	if !ok {
		return cliExitUsage
	}
	stage := "enforce"
	if p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json")); err != nil {
		fmt.Fprintf(stderr, "[policy] WARN %v; trailer-check fails closed to enforce\n", err)
	} else {
		stage = p.InboxTrailerStage()
	}
	closes, refs, violations, err := checkInboxTrailerRange(root, rng)
	if err != nil {
		fmt.Fprintf(stderr, "inbox trailer-check: %v\n", err)
		return cliExitIO
	}
	valid := len(violations) == 0
	if asJSON {
		report := trailerCheckReport{Range: rng, Stage: stage, OK: valid, Closes: closes, Refs: refs, Violations: violations}
		_ = printVerbJSON("inbox trailer-check", report, stdout, stderr)
		if !valid && stage == "enforce" {
			return cliExitFailed
		}
		return cliExitOK
	}
	if valid {
		fmt.Fprintf(stdout, "inbox trailer-check: ok (%s)\n", rng)
		return cliExitOK
	}
	if stage == "enforce" {
		for _, v := range violations {
			fmt.Fprintf(stderr, "inbox trailer-check: %s\n", v)
		}
		return cliExitFailed
	}
	for _, v := range violations {
		fmt.Fprintf(stdout, "inbox trailer-check: shadow: would refuse: %s\n", v)
	}
	return cliExitOK
}
