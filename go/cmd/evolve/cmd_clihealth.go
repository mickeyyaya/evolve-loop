package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func runClihealth(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: evolve clihealth list [--json] | clear <family> [--project-root DIR]")
		return 2
	}
	fs := flag.NewFlagSet("clihealth "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "project root holding .evolve/cli-health.json")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, err := parseInterspersed(fs, args[1:])
	if err != nil {
		return 2
	}
	store := clihealth.NewStore(*root, time.Now)
	switch {
	case args[0] == "list" && len(positional) == 0:
		return clihealthList(store, *asJSON, stdout)
	case args[0] == "clear" && len(positional) == 1:
		return clihealthClear(store, positional[0], stderr)
	}
	fmt.Fprintf(stderr, "evolve clihealth: bad invocation %q; want list [--json] | clear <family>\n", args)
	return 2
}

func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func clihealthList(store *clihealth.Store, asJSON bool, stdout io.Writer) int {
	active := store.Active()
	entries := make([]clihealth.Entry, 0, len(active))
	for _, e := range active {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Family < entries[j].Family })
	if asJSON {
		body, _ := json.Marshal(entries)
		fmt.Fprintln(stdout, string(body))
		return 0
	}
	for _, e := range entries {
		fmt.Fprintf(stdout, "%s\t%s\tuntil %s\n", e.Family, e.Reason, e.BenchedUntil.Format(time.RFC3339))
	}
	return 0
}

func clihealthClear(store *clihealth.Store, family string, stderr io.Writer) int {
	if _, ok := store.Active()[family]; !ok {
		fmt.Fprintf(stderr, "evolve clihealth: family %q is not benched\n", family)
		return 1
	}
	if err := store.Clear(family); err != nil {
		fmt.Fprintf(stderr, "evolve clihealth: %v\n", err)
		return 1
	}
	return 0
}
