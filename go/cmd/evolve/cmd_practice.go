package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/practicewatch"
)

const (
	practiceUsage           = "usage: evolve practice watch [--dry-run] [--json] [--project-root P]"
	practiceUsageExit       = 10
	practiceRefusedExit     = 1
	practiceInterruptedExit = 130
)

var practiceDefaultSources = practicewatch.Defaults

var practiceAbsoluteRoot = paths.AbsoluteRoot

type practiceOptions struct {
	projectRoot string
	dryRun      bool
	asJSON      bool
}

func runPracticeCommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return practiceVerb(ctx, args, stdout, stderr)
}

func practiceVerb(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, rc := parsePracticeArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	rep, err := runPracticeWatch(ctx, opts.projectRoot, opts.dryRun, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve practice watch: refused: %v\n", err)
		return practiceRefusedExit
	}
	printPracticeReport(rep, opts.asJSON, stdout)
	if ctx.Err() != nil {
		fmt.Fprintf(stderr, "evolve practice watch: interrupted (%v); a source not yet reached was not read\n", ctx.Err())
		return practiceInterruptedExit
	}
	return 0
}

func parsePracticeArgs(args []string, stderr io.Writer) (practiceOptions, int) {
	if len(args) == 0 || args[0] != "watch" {
		fmt.Fprintf(stderr, "evolve practice watch: want the watch subcommand (%s)\n", practiceUsage)
		return practiceOptions{}, practiceUsageExit
	}
	fs := flag.NewFlagSet("evolve practice watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	dryRun := fs.Bool("dry-run", false, "fetch and compare, but record and file nothing")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(stderr, "evolve practice watch: "+practiceUsage)
		return practiceOptions{}, practiceUsageExit
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve practice watch: unexpected argument %q (%s)\n", fs.Arg(0), practiceUsage)
		return practiceOptions{}, practiceUsageExit
	}
	return practiceOptions{projectRoot: practiceRoot(*root, stderr), dryRun: *dryRun, asJSON: *asJSON}, 0
}

func practiceRoot(flagValue string, stderr io.Writer) string {
	root := flagValue
	if root == "" {
		root = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if root == "" {
		root = "."
	}
	return practiceAbsoluteRoot("--project-root", root, func(m string) {
		fmt.Fprintf(stderr, "evolve practice watch: WARN: %s\n", m)
	})
}

func runPracticeWatch(ctx context.Context, projectRoot string, dryRun bool, stderr io.Writer) (practicewatch.Report, error) {
	pol, err := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return practicewatch.Report{}, err
	}
	cfg, err := pol.PracticeWatchConfig()
	if err != nil {
		return practicewatch.Report{}, err
	}
	sources, err := practicewatch.Resolve(practiceDefaultSources(), cfg.Sources)
	if err != nil {
		return practicewatch.Report{}, err
	}
	return practicewatch.Watch(ctx, practicewatch.Options{
		Root:    projectRoot,
		Sources: sources,
		Timeout: time.Duration(*cfg.TimeoutS) * time.Second,
		DryRun:  dryRun,
		Inbox: inboxmover.Options{ProjectRoot: projectRoot, Stderr: stderr, IsProtectedPath: laneForbidden(projectRoot, stderr),
			PriorityClasses: pol.InboxPriorityConfig().ClassOrder},
	}), nil
}

func printPracticeReport(rep practicewatch.Report, asJSON bool, stdout io.Writer) {
	if asJSON {
		body, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Fprintln(stdout, string(body))
		return
	}
	if rep.DryRun {
		fmt.Fprintln(stdout, "dry run: nothing was recorded or filed")
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FAMILY\tSOURCE\tKIND\tSTATUS\tDETAIL")
	for _, res := range rep.Results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", res.Family, res.ID, res.Kind, res.Status, res.Detail)
	}
	_ = tw.Flush()
}
