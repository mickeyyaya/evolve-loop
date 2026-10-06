package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
)

const failuresUsage = "evolve failures: usage: failures list [--class C] [--json] | failures reset [--fingerprint F] | failures prune [--dry-run] (prune also drops expired carryoverTodos; reset and a real prune require --project-root)"

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

func parseFailuresFlags(fs *flag.FlagSet, args []string, stderr io.Writer) bool {
	if err := fs.Parse(args); err != nil {
		return false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "%s\nevolve %s: unexpected arguments %q\n", failuresUsage, fs.Name(), fs.Args())
		return false
	}
	return true
}

func failuresReadRoot(root, verb string, stderr io.Writer) (string, bool) {
	if root != "" {
		return root, true
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures %s: cannot resolve the working directory: %v\n", verb, err)
		return "", false
	}
	return wd, true
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
	if !parseFailuresFlags(fs, args, stderr) {
		return 10
	}
	if *class != "" && !isKnownClassification(*class) {
		fmt.Fprintf(stderr, "evolve failures list: unknown class %q\n", *class)
		return 10
	}
	root, ok := failuresReadRoot(*rootFlag, "list", stderr)
	if !ok {
		return 2
	}
	evolveDir := filepath.Join(root, ".evolve")
	entries, err := loadFailedApproaches(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures list: %v\n", err)
		return 2
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
			return 2
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}
	return printFailuresText(evolveDir, shown, stdout, stderr)
}

func printFailuresText(evolveDir string, shown []map[string]any, stdout, stderr io.Writer) int {
	acked, err := core.LoadResolvedFingerprints(evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures list: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "%d failed approach(es)\n", len(shown))
	for _, e := range shown {
		fmt.Fprintf(stdout, "  cycle=%v  %v  %v  expiresAt=%v  %v\n", e["cycle"], e["recordedAt"], e["classification"], fieldOrDash(e, "expiresAt"), e["summary"])
	}
	fingerprints := make([]string, 0, len(acked))
	for fp := range acked {
		fingerprints = append(fingerprints, fp)
	}
	sort.Strings(fingerprints)
	fmt.Fprintf(stdout, "%d acknowledged fingerprint(s)\n", len(fingerprints))
	for _, fp := range fingerprints {
		fmt.Fprintf(stdout, "  %s\n", fp)
	}
	return 0
}

func fieldOrDash(e map[string]any, key string) any {
	if v, ok := e[key]; ok && v != nil && v != "" {
		return v
	}
	return "-"
}

func runFailuresReset(args []string, stdout, stderr io.Writer) int {
	fs, rootFlag := failuresFlags("reset", stderr)
	fingerprint := fs.String("fingerprint", "", "also acknowledge this blocker fingerprint in resolved-fingerprints.json")
	if !parseFailuresFlags(fs, args, stderr) {
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
		code = 2
	} else {
		fmt.Fprintf(stdout, "evolve failures reset: pruned %d failedApproaches (%d→%d)\n", out.pruned.Removed, out.pruned.Before, out.pruned.After)
	}
	if out.ackErr != nil {
		fmt.Fprintf(stderr, "evolve failures reset: --fingerprint: %v\n", out.ackErr)
		code = 2
	} else if out.acked {
		fmt.Fprintf(stdout, "evolve failures reset: acknowledged %q in resolved-fingerprints.json\n", *fingerprint)
	}
	return code
}

func runFailuresPrune(args []string, stdout, stderr io.Writer) int {
	fs, rootFlag := failuresFlags("prune", stderr)
	dryRun := fs.Bool("dry-run", false, "report the expired entries without removing them")
	if !parseFailuresFlags(fs, args, stderr) {
		return 10
	}
	now := time.Now().UTC()
	if *dryRun {
		root, ok := failuresReadRoot(*rootFlag, "prune", stderr)
		if !ok {
			return 2
		}
		return previewFailuresPrune(filepath.Join(root, ".evolve", "state.json"), now, stdout, stderr)
	}
	root, ok := failuresMutatingRoot(*rootFlag, stderr)
	if !ok {
		return 1
	}
	out := pruneExpiredState(filepath.Join(root, ".evolve", "state.json"), now)
	code := 0
	if out.failedErr != nil {
		fmt.Fprintf(stderr, "evolve failures prune: %v\n", out.failedErr)
		code = 2
	} else {
		fmt.Fprintf(stdout, "evolve failures prune: removed %d expired failedApproaches (%d→%d)\n", out.failed.Removed, out.failed.Before, out.failed.After)
	}
	if out.carryoverErr != nil {
		fmt.Fprintf(stderr, "evolve failures prune: carryover: %v\n", out.carryoverErr)
		code = 2
	} else {
		fmt.Fprintf(stdout, "evolve failures prune: removed %d expired carryoverTodos (%d→%d)\n", out.carryover.Removed, out.carryover.Before, out.carryover.After)
	}
	return code
}

func previewFailuresPrune(statePath string, now time.Time, stdout, stderr io.Writer) int {
	failed, carryover, err := failurelog.PreviewExpired(statePath, now)
	if err != nil {
		fmt.Fprintf(stderr, "evolve failures prune: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "evolve failures prune --dry-run: would remove %d expired failedApproaches (%d→%d)\n", failed.Removed, failed.Before, failed.After)
	for _, e := range failed.Expired {
		fmt.Fprintf(stdout, "  failedApproaches cycle=%v %v expiresAt=%v\n", fieldOrDash(e, "cycle"), fieldOrDash(e, "classification"), fieldOrDash(e, "expiresAt"))
	}
	fmt.Fprintf(stdout, "evolve failures prune --dry-run: would remove %d expired carryoverTodos (%d→%d)\n", carryover.Removed, carryover.Before, carryover.After)
	for _, e := range carryover.Expired {
		fmt.Fprintf(stdout, "  carryoverTodos id=%v expiresAt=%v\n", fieldOrDash(e, "id"), fieldOrDash(e, "expiresAt"))
	}
	return 0
}
