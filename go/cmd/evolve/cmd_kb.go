package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/kbgraph"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
)

const kbUsage = `Usage:
  evolve kb graph neighbors <node-id> [--hops N] [--json] [--project-root P]
  evolve kb graph recurrences <fingerprint> [--json] [--project-root P]
  evolve kb recall score [--gold FILE] [--corpus DIR] [--project-root P] [--json]`

const (
	defaultGoldRel   = "go/internal/research/testdata/gold_set.json"
	defaultCorpusRel = "go/internal/research/testdata/gold_corpus"
	defaultRecallK   = 5
)

type kbRecallReport struct {
	Ranker    string  `json:"ranker"`
	K         int     `json:"k"`
	Queries   int     `json:"queries"`
	RecallAt5 float64 `json:"recall_at_5"`
	MRR       float64 `json:"mrr"`
}

func runKB(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "evolve kb: expected `graph neighbors|recurrences` or `recall score`\n"+kbUsage)
		return 10
	}
	switch args[0] {
	case "graph":
		return runKBGraph(args[1], args[2:], stdout, stderr)
	case "recall":
		return runKBRecall(args[1], args[2:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "evolve kb: expected `graph neighbors|recurrences` or `recall score`\n"+kbUsage)
		return 10
	}
}

func runKBGraph(verb string, rest []string, stdout, stderr io.Writer) int {
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

func runKBRecall(verb string, rest []string, stdout, stderr io.Writer) int {
	if verb != "score" {
		fmt.Fprintf(stderr, "evolve kb recall: unknown verb %q\n%s\n", verb, kbUsage)
		return 10
	}
	fs := flag.NewFlagSet("evolve kb recall score", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", ".", "repository root holding go/internal/research/testdata/")
	goldFlag := fs.String("gold", "", "path to gold query set JSON")
	corpusFlag := fs.String("corpus", "", "path to lesson corpus directory")
	asJSON := fs.Bool("json", false, "print JSON instead of lines")
	if fs.Parse(rest) != nil || fs.NArg() != 0 {
		fmt.Fprintln(stderr, kbUsage)
		return 10
	}
	return execKBRecallScore(*root, *goldFlag, *corpusFlag, *asJSON, stdout, stderr)
}

func execKBRecallScore(root, goldPath, corpusDir string, asJSON bool, stdout, stderr io.Writer) int {
	if goldPath == "" {
		goldPath = filepath.Join(root, filepath.FromSlash(defaultGoldRel))
	}
	if corpusDir == "" {
		corpusDir = filepath.Join(root, filepath.FromSlash(defaultCorpusRel))
	}
	info, err := os.Stat(corpusDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "evolve kb recall score: corpus %s is not a readable directory\n", corpusDir)
		return 1
	}
	set, err := research.LoadGoldSet(goldPath)
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb recall score: %v\n", err)
		return 1
	}
	kb := research.NewFileKBWithRecall([]string{corpusDir}, defaultRecallK)
	res, err := research.ScoreRecall(context.Background(), kb, set, defaultRecallK)
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb recall score: %v\n", err)
		return 1
	}
	rep := kbRecallReport{Ranker: "FileKB.Lookup", K: res.K, Queries: res.Queries, RecallAt5: res.Recall, MRR: res.MRR}
	if asJSON {
		return printJSON(rep, stdout, stderr)
	}
	fmt.Fprintf(stdout, "ranker: %s\nqueries: %d\nrecall@5: %.6f\nMRR: %.6f\n", rep.Ranker, rep.Queries, rep.RecallAt5, rep.MRR)
	return 0
}

func printJSON(v any, stdout, stderr io.Writer) int {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "evolve kb: %v\n", err)
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
