package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

const failuresUsage = "evolve failures: usage: failures list [--class C] [--json] | failures reset [--fingerprint F] | failures prune"

func runFailures(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, failuresUsage)
		return 10
	}
	switch args[0] {
	case "list":
		return runFailuresList(args[1:], stdout, stderr)
	case "reset":
		return runFailuresReset(args[1:], stdout, stderr)
	case "prune":
		return runFailuresPrune(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s\nevolve failures: unknown subcommand %q\n", failuresUsage, args[0])
		return 10
	}
}

func failuresFlags(name string, stderr io.Writer) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet("failures "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "repository root `dir` holding .evolve/state.json")
	return fs, root
}

func failuresMutatingRoot(root string, stderr io.Writer) (string, bool) {
	if root == "" {
		fmt.Fprintln(stderr, "evolve failures: mutating run refused: --project-root must be explicitly set")
		return "", false
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures: resolve --project-root: %v\n", err)
		return "", false
	}
	return abs, true
}

func loadFailedApproaches(statePath string) ([]map[string]any, error) {
	raw, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var st struct {
		FailedApproaches []map[string]any `json:"failedApproaches"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	if st.FailedApproaches == nil {
		return []map[string]any{}, nil
	}
	return st.FailedApproaches, nil
}

func isKnownClassification(c string) bool {
	for _, k := range failurelog.KnownClassifications() {
		if string(k) == c {
			return true
		}
	}
	return false
}

func runFailuresList(args []string, stdout, stderr io.Writer) int {
	fs, rootFlag := failuresFlags("list", stderr)
	class := fs.String("class", "", "only entries of this classification")
	asJSON := fs.Bool("json", false, "emit a JSON array")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if *class != "" && !isKnownClassification(*class) {
		fmt.Fprintf(stderr, "evolve failures list: unknown class %q\n", *class)
		return 10
	}
	root := *rootFlag
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve failures list: cannot resolve the working directory: %v\n", err)
			return 1
		}
		root = wd
	}
	entries, err := loadFailedApproaches(filepath.Join(root, ".evolve", "state.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures list: %v\n", err)
		return 1
	}
	shown := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if cls, _ := e["classification"].(string); *class == "" || cls == *class {
			shown = append(shown, e)
		}
	}
	if *asJSON {
		out, err := json.MarshalIndent(shown, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "evolve failures list: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	fmt.Fprintf(stdout, "%d failed approach(es)\n", len(shown))
	for _, e := range shown {
		fmt.Fprintf(stdout, "  cycle=%v  %v  %v  %v\n", e["cycle"], e["recordedAt"], e["classification"], e["summary"])
	}
	return 0
}

func runFailuresReset(args []string, stdout, stderr io.Writer) int {
	fs, rootFlag := failuresFlags("reset", stderr)
	fingerprint := fs.String("fingerprint", "", "also acknowledge this blocker fingerprint in resolved-fingerprints.json")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	root, ok := failuresMutatingRoot(*rootFlag, stderr)
	if !ok {
		return 1
	}
	evolveDir := filepath.Join(root, ".evolve")
	out := resetFailures(filepath.Join(evolveDir, "state.json"), evolveDir, *fingerprint)
	code := 0
	if out.pruneErr != nil {
		fmt.Fprintf(stderr, "evolve failures reset: %v\n", out.pruneErr)
		code = 1
	} else {
		fmt.Fprintf(stdout, "evolve failures reset: pruned %d failedApproaches (%d→%d)\n", out.pruned.Removed, out.pruned.Before, out.pruned.After)
	}
	if out.ackErr != nil {
		fmt.Fprintf(stderr, "evolve failures reset: --fingerprint: %v\n", out.ackErr)
		code = 1
	} else if out.acked {
		fmt.Fprintf(stdout, "evolve failures reset: acknowledged %q in resolved-fingerprints.json\n", *fingerprint)
	}
	return code
}

func runFailuresPrune(args []string, stdout, stderr io.Writer) int {
	fs, rootFlag := failuresFlags("prune", stderr)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	root, ok := failuresMutatingRoot(*rootFlag, stderr)
	if !ok {
		return 1
	}
	pr, err := failurelog.PruneExpired(filepath.Join(root, ".evolve", "state.json"), time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures prune: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "evolve failures prune: removed %d expired failedApproaches (%d→%d)\n", pr.Removed, pr.Before, pr.After)
	return 0
}
