package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type emptyStageReader struct{}

func (emptyStageReader) ReadChain(context.Context, string) (stageevidence.Chain, error) {
	return stageevidence.Chain{}, stageevidence.ErrEmptyChain
}

func runStage(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve stage: usage: evolve stage <verify <key>|report> [--chain PATH] [--project-root P] [--json]")
		return cliExitUsage
	}
	switch args[0] {
	case "verify":
		return runStageVerify(args[1:], stdout, stderr)
	case "report":
		return runStageReport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve stage: unknown subcommand %q\n", args[0])
		return cliExitUsage
	}
}

type stageFlags struct {
	chainPath   string
	projectRoot string
	asJSON      bool
}

func parseStageFlags(verb string, args []string, stderr io.Writer) (stageFlags, []string, int) {
	fs := flag.NewFlagSet("evolve stage "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var f stageFlags
	fs.StringVar(&f.chainPath, "chain", "", "path to stage evidence chain JSON file")
	fs.StringVar(&f.projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&f.asJSON, "json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil {
		return stageFlags{}, nil, cliExitUsage
	}
	return f, fs.Args(), cliExitOK
}

func lookupStageEntry(key string) (gatecontract.Entry, bool) {
	for _, e := range gateregistry.StageEntries() {
		if e.Shadow != nil && e.Shadow.PolicyKey == key {
			return e, true
		}
	}
	return gatecontract.Entry{}, false
}

func loadStagePolicy(projectRoot string, stderr io.Writer) (string, []byte, int) {
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve stage: cwd: %v\n", err)
		return "", nil, cliExitIO
	}
	policyBytes, err := os.ReadFile(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve stage: read policy.json: %v\n", err)
		return "", nil, cliExitIO
	}
	return root, policyBytes, cliExitOK
}

func stageChainReader(chainPath string) stageevidence.ChainReader {
	if chainPath != "" {
		return stageevidence.FileChain(chainPath)
	}
	return emptyStageReader{}
}

func verifyStageEntry(root string, policyBytes []byte, e gatecontract.Entry, reader stageevidence.ChainReader) (stageevidence.Result, int, error) {
	version, err := stageevidence.DecisionVersion(filepath.Join(root, "go"), e.Shadow.Closure, policyBytes, e.Shadow.PolicySubtree)
	if err != nil {
		return stageevidence.Result{}, cliExitIO, err
	}
	res, err := stageevidence.Verify(context.Background(), reader, e.Shadow.PolicyKey, version, e.Shadow.Promotion, stageevidence.ScorerFor(e.Shadow.PolicyKey))
	if err != nil && !errors.Is(err, stageevidence.ErrBrokenChain) && !errors.Is(err, stageevidence.ErrEmptyChain) {
		return res, cliExitIO, err
	}
	return res, cliExitOK, err
}

func runStageVerify(args []string, stdout, stderr io.Writer) int {
	var key string
	flagArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		key = args[0]
		flagArgs = args[1:]
	}
	flags, rest, rc := parseStageFlags("verify", flagArgs, stderr)
	if rc != cliExitOK {
		return rc
	}
	if key == "" {
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "evolve stage verify: usage: evolve stage verify <key> [--chain PATH] [--project-root P] [--json]")
			return cliExitUsage
		}
		key = rest[0]
	} else if len(rest) != 0 {
		fmt.Fprintf(stderr, "evolve stage verify: unexpected argument %q\n", rest[0])
		return cliExitUsage
	}
	entry, ok := lookupStageEntry(key)
	if !ok {
		fmt.Fprintf(stderr, "evolve stage verify: unknown stage key %q\n", key)
		return cliExitUsage
	}
	root, policyBytes, rc := loadStagePolicy(flags.projectRoot, stderr)
	if rc != cliExitOK {
		return rc
	}
	res, rc, verifyErr := verifyStageEntry(root, policyBytes, entry, stageChainReader(flags.chainPath))
	if rc != cliExitOK {
		fmt.Fprintf(stderr, "evolve stage verify: %v\n", verifyErr)
		return rc
	}
	if flags.asJSON {
		if jrc := printVerbJSON("stage verify", res, stdout, stderr); jrc != cliExitOK {
			return jrc
		}
	} else {
		writeStageVerifyText(stdout, res, verifyErr)
	}
	if res.Ready {
		return cliExitOK
	}
	return cliExitFailed
}

func writeStageVerifyText(stdout io.Writer, res stageevidence.Result, verifyErr error) {
	status := "not ready"
	if res.Ready {
		status = "ready"
	}
	fmt.Fprintf(stdout, "%s: %s\n", res.Key, status)
	if res.Error != "" {
		detail := res.Error
		if verifyErr != nil {
			detail = fmt.Sprintf("%s (%v)", res.Error, verifyErr)
		}
		fmt.Fprintf(stdout, "error: %s\n", detail)
	}
	fmt.Fprintf(stdout, "scorer: %s\n", res.Scorer)
	fmt.Fprintf(stdout, "decision version: %s\n", res.Version)
	fmt.Fprintf(stdout, "evidence range: cycle %d to cycle %d\n", res.FirstCycle, res.LastCycle)
	fmt.Fprintf(stdout, "chain digest: %s\n", res.ChainDigest)
	fmt.Fprintf(stdout, "decisions: %d\n", res.Decisions)
	fmt.Fprintf(stdout, "false stops: %d\n", res.FalseStops)
	fmt.Fprintf(stdout, "escapes: %d\n", res.Escapes)
	fmt.Fprintf(stdout, "conflicts: %d\n", res.Conflicts)
	fmt.Fprintf(stdout, "incomplete cycles: %d\n", len(res.IncompleteCycles))
	for _, c := range res.Cases {
		fmt.Fprintf(stdout, "case: %s cycle %d seq %d\n", c.Class, c.Cycle, c.Seq)
	}
	if len(res.OtherVersions) > 0 {
		versions := make([]string, 0, len(res.OtherVersions))
		for v := range res.OtherVersions {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		for _, v := range versions {
			fmt.Fprintf(stdout, "other version: %s (%d records)\n", v, res.OtherVersions[v])
		}
	}
}

func runStageReport(args []string, stdout, stderr io.Writer) int {
	flags, rest, rc := parseStageFlags("report", args, stderr)
	if rc != cliExitOK {
		return rc
	}
	if len(rest) != 0 {
		fmt.Fprintf(stderr, "evolve stage report: unexpected argument %q\n", rest[0])
		return cliExitUsage
	}
	root, policyBytes, rc := loadStagePolicy(flags.projectRoot, stderr)
	if rc != cliExitOK {
		return rc
	}
	reader := stageChainReader(flags.chainPath)
	entries := gateregistry.StageEntries()
	results := make([]stageevidence.Result, 0, len(entries))
	for _, entry := range entries {
		res, rc, err := verifyStageEntry(root, policyBytes, entry, reader)
		if rc != cliExitOK {
			res = stageevidence.Result{
				Key:    entry.Shadow.PolicyKey,
				Scorer: stageevidence.ScorerFor(entry.Shadow.PolicyKey).Name(),
				Error:  stageevidence.ErrorCode(err),
			}
		}
		results = append(results, res)
	}
	if flags.asJSON {
		return printVerbJSON("stage report", results, stdout, stderr)
	}
	for _, res := range results {
		status := "not_ready"
		if res.Ready {
			status = "ready"
		}
		fmt.Fprintf(stdout, "%s\t%s\tscorer=%s\tversion=%s\tdecisions=%d\tfalse_stops=%d\tescapes=%d\tconflicts=%d\terror=%s\n",
			res.Key, status, res.Scorer, res.Version, res.Decisions, res.FalseStops, res.Escapes, res.Conflicts, res.Error)
	}
	return cliExitOK
}
