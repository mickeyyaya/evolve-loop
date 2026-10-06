package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func runInboxAdd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	raw, rc := readInboxAddItem(args, stdin, stderr)
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
		PriorityClasses: pol.InboxPriorityConfig().ClassOrder}
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

func readInboxAddItem(args []string, stdin io.Reader, stderr io.Writer) ([]byte, int) {
	source := stdin
	switch {
	case len(args) == 0 && stdin != nil:
	case len(args) == 2 && args[0] == "--file":
		f, err := os.Open(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "inbox add: %v\n", err)
			return nil, 2
		}
		defer func() { _ = f.Close() }()
		source = f
	default:
		fmt.Fprintln(stderr, inboxUsage("add"))
		return nil, 10
	}
	raw, err := io.ReadAll(source)
	if err != nil {
		fmt.Fprintf(stderr, "inbox add: read the item: %v\n", err)
		return nil, 2
	}
	return raw, 0
}

func dispatchability(consoleReason string) string {
	if consoleReason != "" {
		return "console-owned: " + consoleReason
	}
	return "lane-dispatchable"
}
