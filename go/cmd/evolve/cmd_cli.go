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
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const cliUsage = "usage: evolve cli update [--dry-run] [--json] [--project-root P]"

const liveCheckPaneLines = 6

type cliUpdateWiring struct {
	families []cliupdate.Family
	seams    cliupdate.Seams
	now      func() time.Time
}

var cliUpdateWiringFn = productionCLIUpdateWiring

type cliUpdateOptions struct {
	projectRoot string
	dryRun      bool
	asJSON      bool
}

func runCLICommand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cliUpdateVerb(ctx, args, stdout, stderr)
}

func cliUpdateVerb(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, rc := parseCLIUpdateArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	evolveDir := filepath.Join(opts.projectRoot, ".evolve")
	if live := runlease.LiveRuns(filepath.Join(evolveDir, "runs"), time.Now()); len(live) > 0 && !opts.dryRun {
		fmt.Fprintf(stderr, "evolve cli update: refused: %d run(s) hold a live lease (%s); CLIs stay frozen within a wave, so update at a boundary (the loop does it itself) or pass --dry-run\n", len(live), live[0].Dir)
		return 2
	}
	rep := updateCLIs(ctx, opts.projectRoot, evolveDir, opts.dryRun, stderr)
	printCLIUpdateReport(rep, opts.asJSON, stdout)
	switch {
	case ctx.Err() != nil:
		fmt.Fprintf(stderr, "evolve cli update: interrupted (%v); a family not yet reached was skipped\n", ctx.Err())
		return 130
	case len(rep.Failed()) > 0:
		return 1
	}
	return 0
}

func parseCLIUpdateArgs(args []string, stderr io.Writer) (cliUpdateOptions, int) {
	if len(args) == 0 || args[0] != "update" {
		fmt.Fprintf(stderr, "evolve cli update: want the update subcommand (%s)\n", cliUsage)
		return cliUpdateOptions{}, 2
	}
	fs := flag.NewFlagSet("evolve cli update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	dryRun := fs.Bool("dry-run", false, "report what would run and change nothing")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(stderr, "evolve cli update: "+cliUsage)
		return cliUpdateOptions{}, 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve cli update: unexpected argument %q (%s)\n", fs.Arg(0), cliUsage)
		return cliUpdateOptions{}, 2
	}
	return cliUpdateOptions{projectRoot: cliUpdateRoot(*root, stderr), dryRun: *dryRun, asJSON: *asJSON}, 0
}

func cliUpdateRoot(flagValue string, stderr io.Writer) string {
	root := flagValue
	if root == "" {
		root = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if root == "" {
		root = "."
	}
	return paths.AbsoluteRoot("--project-root", root, func(m string) {
		fmt.Fprintf(stderr, "evolve cli update: WARN: %s\n", m)
	})
}

func updateCLIs(ctx context.Context, projectRoot, evolveDir string, dryRun bool, log io.Writer) cliupdate.Report {
	w := cliUpdateWiringFn(projectRoot, log)
	if dryRun {
		return cliupdate.Plan(w.families)
	}
	history, err := cliupdate.LoadRecords(evolveDir)
	if err != nil {
		fmt.Fprintf(log, "[cli-update] WARN: could not read %s: %v; a version change between boundaries cannot be found this time\n", cliupdate.RecordsPath(evolveDir), err)
	}
	rep := cliupdate.Update(ctx, w.families, w.seams, history)
	if err := cliupdate.Remember(evolveDir, rep, w.now()); err != nil {
		fmt.Fprintf(log, "[cli-update] WARN: could not record the boundary updates in %s: %v; cli-version-drift will warn on them\n", cliupdate.RecordsPath(evolveDir), err)
	}
	return rep
}

func printCLIUpdateReport(rep cliupdate.Report, asJSON bool, stdout io.Writer) {
	if asJSON {
		body, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Fprintln(stdout, string(body))
		return
	}
	if rep.DryRun {
		fmt.Fprintln(stdout, "dry run: nothing was probed, updated or recorded")
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FAMILY\tSTATUS\tVERSION\tDETAIL")
	for _, res := range rep.Results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", res.Family, res.Status, res.Versions(), res.Detail)
	}
	_ = tw.Flush()
}

func productionCLIUpdateWiring(projectRoot string, log io.Writer) cliUpdateWiring {
	live := liveCheck(func(ctx context.Context) liveProbe { return defaultLiveProbe(ctx, projectRoot, log) })
	return cliUpdateWiring{
		families: manifestUpdateFamilies(bridge.InteractiveFamilies(), bridge.LoadManifest, log),
		seams: cliupdate.Seams{
			Eligible: cliupdate.Unwalled(credentialWall(clihealth.NewStore(projectRoot, nil))),
			Probe:    live,
			Version:  looppreflight.CLIVersion,
			Run:      cliupdate.Exec(cliupdate.GroupRunner),
			Smoke:    live,
			Explain:  familyEvidence(usageEvidenceFn(projectRoot, filepath.Join(projectRoot, ".evolve"), log)),
		},
		now: time.Now,
	}
}

func manifestUpdateFamilies(installed []string, load func(string) (bridge.Manifest, error), log io.Writer) []cliupdate.Family {
	out := make([]cliupdate.Family, 0, len(installed))
	for _, family := range installed {
		m, err := load(family + "-tmux")
		if err != nil {
			fmt.Fprintf(log, "[cli-update] WARN: %s is not updated: its manifest did not load: %v\n", family, err)
			continue
		}
		out = append(out, cliupdate.Family{Name: family, UpdateArgv: m.UpdateArgv, AutoUpdateOffEnv: m.AutoUpdateOffEnv})
	}
	return out
}

func credentialWall(store *clihealth.Store) func(family string) (string, bool) {
	return func(family string) (string, bool) {
		e, benched := store.Active()[family]
		if !benched || e.Reason != clihealth.CredentialPattern {
			return "", false
		}
		return e.OperatorAction, true
	}
}

func liveCheck(newProbe func(context.Context) liveProbe) func(ctx context.Context, family string) error {
	return func(ctx context.Context, family string) error {
		driver := family + "-tmux"
		rc, pattern, scrollback := newProbe(ctx)(driver)
		return liveCheckErr(driver, liveProbeResult{rc: rc, pattern: pattern, scrollback: scrollback})
	}
}

type liveProbeResult struct {
	rc                  int
	pattern, scrollback string
}

func liveCheckErr(driver string, r liveProbeResult) error {
	switch {
	case r.rc == bridge.ExitOK:
		return nil
	case r.rc == bridge.ExitREPLBootTimeout && r.pattern == "":
		return fmt.Errorf("doctor live %s: %w after %d attempt(s); final pane: %s", driver, cliupdate.ErrBootTimeout, bridge.ProbeBootAttempts(driver), paneTailLine(r.scrollback))
	}
	return fmt.Errorf("doctor live %s rc=%d pattern=%q", driver, r.rc, r.pattern)
}

func paneTailLine(scrollback string) string {
	tail := bridge.ScrollbackTail(scrollback, liveCheckPaneLines)
	if tail == "" {
		return "(no pane was captured)"
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(tail, "\n", " | ")), " ")
}
