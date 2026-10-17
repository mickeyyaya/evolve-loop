package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex/usagelog"
)

const (
	codeUsageVerbName = "usage"
	codeUsageNoPhase  = "(none)"
)

type codeCall struct {
	verb   string
	root   string
	argLen int
	hits   int
}

type codeUsageSink struct {
	env      func(string) string
	cacheDir func() (string, error)
	rootOf   func() string
}

func codeVerbLogged(ctx context.Context, args []string, stdout, stderr io.Writer, build codeBackendsBuilder, sink codeUsageSink) int {
	started := time.Now()
	rc, call := codeVerb(ctx, args, stdout, stderr, build)
	if call.verb == "" {
		return rc
	}
	phase, cycle, cli := usagelog.Identity(sink.env)
	cwd, _ := os.Getwd()
	rec := usagelog.Record{
		TS: started, Command: call.verb, ArgLen: call.argLen, Exit: rc, Hits: call.hits,
		DurationMS: time.Since(started).Milliseconds(), Phase: phase, Cycle: cycle, CLI: cli, Cwd: cwd,
	}
	if err := appendCodeUsage(sink, cmp.Or(call.root, sink.rootOf()), rec); err != nil {
		fmt.Fprintf(stderr, "evolve code: usage log: %v\n", err)
	}
	return rc
}

func appendCodeUsage(sink codeUsageSink, projectRoot string, rec usagelog.Record) error {
	path, rotateAt, err := usagelog.Target(sink.env, projectRoot, sink.cacheDir)
	if err != nil {
		return err
	}
	return usagelog.Append(path, rec, rotateAt)
}

func codeUsageVerb(args []string, stdout, stderr io.Writer, cacheDir func() (string, error), projectRoot string) int {
	fs := flag.NewFlagSet("evolve code usage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	since := fs.Duration("since", 0, "count only the calls of the last D (a Go duration such as 24h); 0 counts all")
	asJSON := fs.Bool("json", false, "print the rows as a JSON list")
	if err := fs.Parse(args); err != nil {
		return codeExitUsage
	}
	if fs.NArg() != 0 || *since < 0 {
		fmt.Fprintf(stderr, "evolve code usage: want no argument and a non-negative --since\n%s\n", codeUsage)
		return codeExitUsage
	}
	var from time.Time
	if *since > 0 {
		from = time.Now().Add(-*since)
	}
	userPath, err := usagelog.UserPath(cacheDir)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code usage: %v\n", err)
		return codeExitFailed
	}
	records, skipped, err := usagelog.ReadAll(userPath, projectRoot, from)
	if err != nil {
		fmt.Fprintf(stderr, "evolve code usage: %v\n", err)
		return codeExitFailed
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "evolve code usage: skipped %d malformed line(s) of the usage log\n", skipped)
	}
	printCodeUsage(usagelog.Summarize(records), *asJSON, stdout)
	return 0
}

func printCodeUsage(rows []usagelog.Row, asJSON bool, stdout io.Writer) {
	if asJSON {
		_ = json.NewEncoder(stdout).Encode(rows)
		return
	}
	fmt.Fprintln(stdout, "PHASE\tCOMMAND\tCALLS\tHITS\tFAILURES")
	for _, row := range rows {
		phase := row.Phase
		if phase == "" {
			phase = codeUsageNoPhase
		}
		fmt.Fprintf(stdout, "%s\t%s\t%d\t%d\t%d\n", phase, row.Command, row.Calls, row.Hits, row.Failures)
	}
}
