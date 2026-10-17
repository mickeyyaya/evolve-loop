package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/kbgraph"
)

const kbUsage = `Usage:
  evolve kb graph neighbors <node-id> [--hops N] [--json] [--project-root P]
  evolve kb graph recurrences <fingerprint> [--json] [--project-root P]`

func runKB(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "graph" {
		fmt.Fprintln(stderr, "evolve kb: expected `graph neighbors|recurrences`\n"+kbUsage)
		return 10
	}
	verb, rest := args[1], args[2:]
	if verb != "neighbors" && verb != "recurrences" {
		fmt.Fprintf(stderr, "evolve kb graph: unknown verb %q\n%s\n", verb, kbUsage)
		return 10
	}
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		fmt.Fprintln(stderr, kbUsage)
		return 10
	}
	subject := rest[0]
	fs := flag.NewFlagSet("evolve kb graph "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "repository root holding knowledge-base/ and .evolve/inbox/")
	hops := fs.Int("hops", 1, "hops to follow from the node, capped at "+strconv.Itoa(kbgraph.MaxHops))
	asJSON := fs.Bool("json", false, "print JSON instead of lines")
	if fs.Parse(rest[1:]) != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, kbUsage)
		return 10
	}
	g, err := kbgraph.Build(*root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb graph: %v\n", err)
		return 1
	}
	if verb == "recurrences" {
		return printRecurrences(g.Recurrences(subject), *asJSON, stdout, stderr)
	}
	sub, err := g.Neighbors(subject, *hops)
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb graph neighbors: %v\n", err)
		return 1
	}
	return printNeighbors(sub, *asJSON, stdout, stderr)
}

func printJSON(v any, stdout, stderr io.Writer) int {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb graph: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(body))
	return 0
}

func printRecurrences(rec kbgraph.Recurrence, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		return printJSON(rec, stdout, stderr)
	}
	cycles := make([]string, len(rec.Cycles))
	for i, c := range rec.Cycles {
		cycles[i] = strconv.Itoa(c)
	}
	fmt.Fprintf(stdout, "fingerprint %s: %d cycle(s) [%s]\n", rec.Fingerprint, rec.Count, strings.Join(cycles, " "))
	return 0
}

func printNeighbors(sub kbgraph.Subgraph, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		return printJSON(sub, stdout, stderr)
	}
	for _, n := range sub.Nodes {
		fmt.Fprintf(stdout, "node %s (%s)\n", n.ID, n.Kind)
	}
	for _, e := range sub.Edges {
		fmt.Fprintf(stdout, "edge %s -%s-> %s\n", e.From, e.Kind, e.To)
	}
	for _, k := range sub.Absent {
		fmt.Fprintf(stdout, "ABSENT %s\n", k)
	}
	if sub.Truncated {
		fmt.Fprintf(stdout, "TRUNCATED at %d nodes\n", kbgraph.MaxNodes)
	}
	return 0
}
