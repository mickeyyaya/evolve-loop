package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func curationOptions(root string, stderr io.Writer) inboxmover.Options {
	return inboxmover.Options{ProjectRoot: root, Stderr: stderr, IsProtectedPath: laneForbidden(root, stderr)}
}

func refusedMidWaveAt(root, verb string, stderr io.Writer) bool {
	runsDir := filepath.Join(paths.EvolveDirOf(root), "runs")
	live := runlease.LiveRuns(runsDir, time.Now())
	if len(live) == 0 {
		return false
	}
	fmt.Fprintf(stderr, "inbox %s: refused — a loop lane is live (%s, pid %d); the item is a tracked file, so curate it at a wave boundary (`evolve loop-stop --wait`)\n",
		verb, live[0].Dir, live[0].Lease.OwnerPID)
	return true
}

func curationExitCode(verb string, err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "inbox %s: %v\n", verb, err)
	switch {
	case errors.Is(err, inboxmover.ErrBadArgs):
		return 10
	case errors.Is(err, inboxmover.ErrNotFound), errors.Is(err, inboxmover.ErrInvalidItem), errors.Is(err, inboxmover.ErrNotWithdrawable),
		errors.Is(err, inboxmover.ErrConsoleRouted):
		return 1
	}
	return 2
}

func runInboxEdit(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
	target, edits, ok := parseInboxEditArgs(args, stderr)
	if !ok {
		fmt.Fprintln(stderr, inboxUsage("edit"))
		return 10
	}
	if rc, queued := inboxQueueGate(root, "edit", args, asJSON, stdout, stderr); queued {
		return rc
	}
	path, err := inboxmover.Edit(curationOptions(root, stderr), target, edits)
	if rc := curationExitCode("edit", err, stderr); rc != 0 {
		return rc
	}
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "edit", Path: path}, "inbox edit: edited "+filepath.Base(path), stdout, stderr)
}

func parseInboxEditArgs(args []string, stderr io.Writer) (string, []inboxmover.FieldEdit, bool) {
	var edits []inboxmover.FieldEdit
	fs := flag.NewFlagSet("inbox edit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	for _, op := range []inboxmover.EditOp{inboxmover.EditSet, inboxmover.EditAdd, inboxmover.EditRemove} {
		fs.Func(string(op), string(op)+" a curation field: F=V", func(pair string) error {
			field, value, found := strings.Cut(pair, "=")
			if !found || strings.TrimSpace(field) == "" {
				return fmt.Errorf("%q is not F=V", pair)
			}
			edits = append(edits, inboxmover.FieldEdit{Op: op, Field: field, Value: value})
			return nil
		})
	}
	target, ok := parseOnePositional(fs, args)
	return target, edits, ok && len(edits) > 0
}

func parseOnePositional(fs *flag.FlagSet, args []string) (string, bool) {
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return "", false
	}
	positional := strings.TrimSpace(fs.Arg(0))
	if err := fs.Parse(fs.Args()[1:]); err != nil || fs.NArg() > 0 {
		return "", false
	}
	return positional, positional != ""
}

func runInboxWithdraw(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
	if len(args) != 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, inboxUsage("withdraw"))
		return 10
	}
	if rc, queued := inboxQueueGate(root, "withdraw", args, asJSON, stdout, stderr); queued {
		return rc
	}
	id := strings.TrimSpace(args[0])
	path, err := inboxmover.Withdraw(curationOptions(root, stderr), id, args[1])
	if rc := curationExitCode("withdraw", err, stderr); rc != 0 {
		return rc
	}
	human := fmt.Sprintf("inbox withdraw: %s withdrawn (%s removed; the id may be filed again)", id, filepath.Base(path))
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "withdraw", ID: id, Path: path}, human, stdout, stderr)
}

func runInboxVerify(root string, args []string, stdout, stderr io.Writer) int {
	asJSON, args := takeInboxJSON(args)
	fs := flag.NewFlagSet("inbox verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	evidence := fs.String("evidence", "", "what the console checked, and what it found")
	id, ok := parseOnePositional(fs, args)
	if !ok || strings.TrimSpace(*evidence) == "" {
		fmt.Fprintln(stderr, inboxUsage("verify"))
		return 10
	}
	if rc, queued := inboxQueueGate(root, "verify", args, asJSON, stdout, stderr); queued {
		return rc
	}
	path, err := inboxmover.VerifyPremise(curationOptions(root, stderr), id, *evidence)
	if rc := curationExitCode("verify", err, stderr); rc != 0 {
		return rc
	}
	human := fmt.Sprintf("inbox verify: stamped the premise of %s (%s)", id, filepath.Base(path))
	return finishInboxWrite(asJSON, inboxWriteResult{Verb: "verify", ID: id, Path: path}, human, stdout, stderr)
}
