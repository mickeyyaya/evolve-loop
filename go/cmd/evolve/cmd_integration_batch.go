package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	integrationBatchUsage  = "usage: evolve integration batch [--dry-run] [--main-red-fix] [--project-root P] [PR...]"
	integrationRecordUsage = "usage: evolve integration record-pr --pr N (--reviewed | --class NAME)"
	inboxDir               = ".evolve/inbox/"
	mainRedFixFlag         = "--main-red-fix"
	batchHeldCode          = signalcenter.Code("SHIP_INTEGRATION_BATCH_HELD")
)

var reviewClasses = map[string]func(paths []string) bool{
	"docs-review": docsOnlyData,
}

type batchPorts struct {
	Repo integration.Repo
	CI   integration.GhCI
	Main integration.GitMain
	Runs *integration.RunLog
}

func (e integrationEnv) ports() batchPorts {
	repo := integration.Repo{Dir: e.Root, Git: e.Git}
	runs := integration.NewRunLog()
	return batchPorts{Repo: repo, CI: integration.GhCI{Repo: repo, Gh: e.Gh, Settle: e.Settle, Runs: runs}, Main: integration.GitMain{Repo: repo}, Runs: runs}
}

func (e integrationEnv) train(projectRoot string, warn io.Writer) (integration.Train, *integration.RunLog) {
	p := e.ports()
	gate := e.stopGate(projectRoot, warn)
	return integration.Train{
		Composer:    integration.Composer{Repo: p.Repo, Stage: policy.FleetIntegrationShadow, Regen: e.regenPort(p.Repo, projectRoot, warn)},
		L2:          e.batchTest(p.Repo, projectRoot, warn),
		CI:          gate.CI(p.CI),
		Main:        gate.Main(p.Main),
		Host:        integration.GhHost{CI: p.CI},
		Inbox:       inboxCloser(e.Root),
		Root:        e.Root,
		Wave:        e.Wave,
		HeadCI:      integration.PeekCI{CI: p.CI},
		MainCI:      p.CI,
		PullRefWait: pullRefWait(projectRoot, warn),
	}, p.Runs
}

func pullRefWait(projectRoot string, warn io.Writer) time.Duration {
	wait, err := policy.LoadOrDefaults(filepath.Join(paths.EvolveDirOf(projectRoot), "policy.json"), warn).PullRefWait()
	if err != nil {
		fmt.Fprintf(warn, "%sthe pull-ref wait is invalid, so it uses the default: %v\n", integrationPrefix, err)
		return policy.DefaultPullRefWaitS * time.Second
	}
	return wait
}

func inboxCloser(root string) integration.InboxFunc {
	return func(_ context.Context, ids []string) ([]string, error) {
		return inboxmover.ReconcileSuperseded(inboxmover.Options{ProjectRoot: root}, ids, "processed", inboxmover.PromoteOpts{})
	}
}

func fixItemCloser(root string, warn io.Writer) func(id string) error {
	return func(id string) error {
		return inboxmover.ConsumeFixItem(inboxmover.Options{ProjectRoot: root, Stderr: warn}, id, inboxmover.PromoteOpts{})
	}
}

type cliIO struct {
	stdout io.Writer
	stderr io.Writer
}

func (c cliIO) fail(err error) int {
	fmt.Fprintf(c.stderr, "%s%v\n", integrationPrefix, err)
	var mainRefusal *integration.MainRefusal
	if errors.As(err, &mainRefusal) {
		fmt.Fprintln(c.stderr, mainRefusalHint(mainRefusal))
	}
	return refusalExit(err)
}

func mainRefusalHint(r *integration.MainRefusal) string {
	if r.Reason == string(integration.MainPending) {
		return "Next: wait for the CI run of main to finish, then run the batch again"
	}
	return fmt.Sprintf("Next: rerun the failed job of main when the red is a flake, or land the fix alone with %s <PR>", mainRedFixFlag)
}

func usageFail(stderr io.Writer, err error, usage string) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", integrationPrefix, err, usage)
	return exitUsage
}

func batchVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var dry, help, redFix bool
	var rootFlag string
	operands, err := cmdutil.Flags().Bools(map[string]*bool{"--dry-run": &dry, "--help": &help, mainRedFixFlag: &redFix}).Values(map[string]*string{"--project-root": &rootFlag}).Parse(args)
	if help && err == nil {
		fmt.Fprintln(stdout, integrationBatchUsage)
		return 0
	}
	var prs []int
	for _, o := range operands {
		n, convErr := strconv.Atoi(o)
		if convErr != nil || n <= 0 {
			err = errors.Join(err, fmt.Errorf("%q is not a PR number", o))
			continue
		}
		prs = append(prs, n)
	}
	if redFix && len(operands) != 1 {
		err = errors.Join(err, errors.New(mainRedFixFlag+" lands exactly one named PR"))
	}
	if err != nil {
		return usageFail(stderr, err, integrationBatchUsage)
	}
	root, rc := env.projectRoot("integration batch", rootFlag, stderr)
	if rc != 0 {
		return rc
	}
	t, runs := env.train(root, stderr)
	t.MainRedFix, t.Progress = redFix, batchProgress(stdout, runs, stopWiring{env: env, projectRoot: root, warn: stderr})
	if dry {
		t.MainCI = integration.PeekCI{CI: env.ports().CI}
	}
	return runBatch(t, batchRequest{prs: prs, dry: dry}, cliIO{stdout, stderr})
}

func batchProgress(w io.Writer, runs *integration.RunLog, signals stopWiring) func(integration.Step) {
	return func(s integration.Step) {
		fmt.Fprintln(w, stepLine(s, runs))
		if s.Kind == integration.StepHold {
			if !baseCheckFailed(s) {
				signals.emit(holdEvent(s, runs))
			}
		}
	}
}

func baseCheckFailed(s integration.Step) bool {
	var refusal *integration.MainRefusal
	return s.Err != nil && !errors.As(s.Err, &refusal)
}

func mainNotGreen(base string) string {
	return fmt.Sprintf("main %s is not green", integration.ShortSHA(base))
}

func holdEvent(s integration.Step, runs *integration.RunLog) signalcenter.Event {
	return signalcenter.Event{
		Origin: batchEventOrigin, Kind: signalcenter.KindShipWarning, Code: batchHeldCode, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("batch PR #%d is held: %s", s.Batch, mainNotGreen(s.Base)),
		Fields: map[string]string{"base": s.Base, "base_run": runText(runs, s.Base), "batch": strconv.Itoa(s.Batch), "prs": joinInts(s.PRs)},
	}
}

func runText(runs *integration.RunLog, tip string) string {
	if id := runs.Run(tip); id != "" {
		return id
	}
	return "none"
}

func stepLine(s integration.Step, runs *integration.RunLog) string {
	prs := joinInts(s.PRs)
	run := runText(runs, s.Tip)
	switch s.Kind {
	case integration.StepTest:
		return fmt.Sprintf("step: test PRs %s in batch PR #%d", prs, s.Batch)
	case integration.StepVerdict:
		return fmt.Sprintf("step: batch PR #%d of PRs %s is %s (run %s)", s.Batch, prs, s.Verdict, run)
	case integration.StepRerun:
		return fmt.Sprintf("step: rerun the failed jobs of batch PR #%d (run %s)", s.Batch, run)
	case integration.StepSplit:
		return fmt.Sprintf("step: split PRs %s in halves", prs)
	case integration.StepLand:
		return fmt.Sprintf("step: landed PRs %s through batch PR #%d (run %s)", prs, s.Batch, run)
	case integration.StepHold:
		if baseCheckFailed(s) {
			return fmt.Sprintf("step: stop PRs %s in batch PR #%d: the check of main %s failed: %v", prs, s.Batch, integration.ShortSHA(s.Base), s.Err)
		}
		return fmt.Sprintf("step: hold PRs %s in batch PR #%d: %s (base run %s)", prs, s.Batch, mainNotGreen(s.Base), runText(runs, s.Base))
	}
	return fmt.Sprintf("step: %s PRs %s in batch PR #%d", s.Kind, prs, s.Batch)
}

type batchRequest struct {
	prs []int
	dry bool
}

func runBatch(t integration.Train, req batchRequest, c cliIO) int {
	ctx := context.Background()
	prs := req.prs
	if !req.dry {
		settled, err := t.Settle(ctx)
		printSettled(c.stdout, settled)
		if err != nil {
			return c.fail(err)
		}
		prs = withoutSettled(prs, settled)
	}
	named := len(req.prs) > 0
	if !named {
		var err error
		if prs, err = t.ReadyPRs(ctx); err != nil {
			return c.fail(err)
		}
	}
	plan, err := t.Plan(ctx, prs)
	if err != nil {
		return c.fail(err)
	}
	printPlan(c.stdout, plan)
	if req.dry {
		fmt.Fprintln(c.stdout, "dry run: nothing was pushed")
		return 0
	}
	out, err := t.Land(ctx, plan)
	fmt.Fprintf(c.stdout, "landed: %s\n", joinInts(out.Landed))
	printLanding(c.stdout, out)
	return c.landExit(out, err)
}

func withoutSettled(prs []int, settled []integration.Settlement) []int {
	return slices.DeleteFunc(slices.Clone(prs), func(pr int) bool {
		return slices.ContainsFunc(settled, func(s integration.Settlement) bool { return s.PR == pr })
	})
}

func printSettled(w io.Writer, settled []integration.Settlement) {
	var inbox []string
	for _, s := range settled {
		fmt.Fprintln(w, s.Line())
		inbox = append(inbox, s.Moved...)
	}
	printLanding(w, integration.Outcome{Inbox: inbox})
}

func (c cliIO) landExit(out integration.Outcome, err error) int {
	var ejected []error
	for _, pr := range out.Ejected {
		ejected = append(ejected, &integration.Refusal{PR: pr, Reason: "was ejected and stays open: read the brief on the PR"})
	}
	switch {
	case err != nil:
		return c.fail(errors.Join(append([]error{err}, ejected...)...))
	case len(ejected) > 0:
		return c.fail(errors.Join(ejected...))
	}
	return 0
}

func printLanding(w io.Writer, out integration.Outcome) {
	if out.Flaked {
		fmt.Fprintln(w, "ci: the first run was red and the rerun was green (a flake)")
	}
	if len(out.Inbox) == 0 {
		return
	}
	files := make([]string, len(out.Inbox))
	for i, id := range out.Inbox {
		files[i] = inboxDir + id + ".json"
	}
	ids := strings.Join(out.Inbox, " ")
	fmt.Fprintf(w, "inbox: moved to processed in this checkout only: %s\n", ids)
	fmt.Fprintf(w, "inbox: to put the move on main, run: evolve ship --class manual --paths %s \"chore: close inbox items %s\"\n", strings.Join(files, ","), ids)
}

func refusalExit(err error) int {
	var refusal *integration.Refusal
	var mainRefusal *integration.MainRefusal
	if errors.As(err, &refusal) || errors.As(err, &mainRefusal) || errors.Is(err, selfheal.ErrMainRedHold) {
		return exitRefused
	}
	return exitIO
}

func printPlan(w io.Writer, p integration.Plan) {
	fmt.Fprintf(w, "main: %s CI %s\n", integration.ShortSHA(p.Main.SHA), mainStateText(p.Main.State))
	fmt.Fprintf(w, "order: %s\n", joinInts(p.Order))
	for _, e := range p.Ejected {
		fmt.Fprintf(w, "eject %s %s %s\n", e.Label(), e.Cause, strings.Join(e.Paths, " "))
	}
	fmt.Fprintf(w, "batch: %d PRs\n", len(p.Kept))
}

func mainStateText(state integration.MainCIState) string {
	if state == integration.MainRedOverridden {
		return "red, " + mainRedFixFlag
	}
	return string(state)
}

func joinInts(items []int) string {
	parts := make([]string, len(items))
	for i, n := range items {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, " ")
}

func recordPRVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var help, reviewed bool
	var pr, class string
	operands, err := cmdutil.Flags().Bools(map[string]*bool{"--help": &help, "--reviewed": &reviewed}).Values(map[string]*string{"--pr": &pr, "--class": &class}).Parse(args)
	if help && err == nil {
		fmt.Fprintln(stdout, integrationRecordUsage)
		return 0
	}
	n, convErr := strconv.Atoi(pr)
	switch {
	case err != nil:
	case len(operands) > 0:
		err = fmt.Errorf("unexpected operand %q", operands[0])
	case convErr != nil || n <= 0:
		err = fmt.Errorf("--pr %q must be a PR number", pr)
	case reviewed == (class != ""):
		err = errors.New("name exactly one of --reviewed and --class")
	}
	if err != nil {
		return usageFail(stderr, err, integrationRecordUsage)
	}
	c := cliIO{stdout, stderr}
	bind := classBinder(class)
	if reviewed {
		store, err := commitgate.ReviewStoreDir(context.Background(), sysexec.DefaultRunner, env.Root)
		if err != nil {
			return c.fail(err)
		}
		bind = reviewedBinder(store)
	}
	t, _ := env.train(env.Root, stderr)
	return recordPR(t, recordRequest{pr: n, bind: bind}, c)
}

type missingRecordError struct{ path string }

func (e *missingRecordError) Error() string { return "no review record at " + e.path }

type reviewedRecord struct{ artifact string }

func reviewedChange(store, digest string, paths []string) (reviewedRecord, error) {
	label := integration.ShortSHA(digest)
	path := filepath.Join(store, digest+".json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return reviewedRecord{}, &missingRecordError{path: path}
	}
	if err != nil {
		return reviewedRecord{}, fmt.Errorf("read the record of change %s at %s: %w", label, path, err)
	}
	var rec commitgate.ReviewRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		reason := fmt.Sprintf("the record of change %s at %s does not parse: %v. Next: run the review seats and evolve commit-gate run again, then run record-pr again.", label, path, err)
		return reviewedRecord{}, errors.New(reason)
	}
	if reason := commitgate.CheckReviewedChange(rec, label, paths); reason != "" {
		return reviewedRecord{}, errors.New(reason)
	}
	if err := integration.CoversChange(rec.Attestation.TreeStateSHA, digest); err != nil {
		return reviewedRecord{}, err
	}
	return reviewedRecord{artifact: integration.Digest(raw)}, nil
}

func reviewedBinder(store string) integration.Binder {
	return func(ev integration.Evidence) (integration.AuditBinding, error) {
		whole, err := reviewedChange(store, ev.DiffSHA, ev.Paths)
		var missing *missingRecordError
		switch {
		case err == nil:
			return integration.AttestedBinding(ev, whole.artifact)
		case !errors.As(err, &missing):
			return integration.AuditBinding{}, err
		case len(ev.Changes) > 1:
			return integration.ChainedBinding(ev, chainCheck(store))
		}
		base, head := integration.ShortSHA(ev.Base), integration.ShortSHA(ev.Commits[len(ev.Commits)-1])
		reason := fmt.Sprintf("no review record for its whole change %s (%s..%s), looked in %s. %s", integration.ShortSHA(ev.DiffSHA), base, head, store, integration.RestoreStep(base, head))
		return integration.AuditBinding{}, errors.New(reason)
	}
}

func chainCheck(store string) integration.ChangeCheck {
	return func(c integration.CommitChange) (string, error) {
		rec, err := reviewedChange(store, c.DiffSHA, c.Paths)
		return rec.artifact, err
	}
}

var instructionRoots = []string{"agents/", "skills/", ".claude/", "commands/", "guidance/"}

var instructionFiles = []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"}

func docsOnlyData(paths []string) bool {
	return commitgate.DocsOnly(paths) && !slices.ContainsFunc(paths, isInstructionPath)
}

func isInstructionPath(p string) bool {
	return slices.ContainsFunc(instructionRoots, func(root string) bool { return strings.HasPrefix(p, root) }) || slices.Contains(instructionFiles, path.Base(p))
}

func classBinder(class string) integration.Binder {
	return func(ev integration.Evidence) (integration.AuditBinding, error) {
		covers, ok := reviewClasses[class]
		if !ok {
			return integration.AuditBinding{}, fmt.Errorf("%q is not a review class (known: %s)", class, strings.Join(slices.Sorted(maps.Keys(reviewClasses)), ", "))
		}
		return integration.ClassBinding(ev, class, covers)
	}
}

type recordRequest struct {
	pr   int
	bind integration.Binder
}

func recordPR(t integration.Train, req recordRequest, c cliIO) int {
	path, err := t.Record(context.Background(), req.pr, req.bind)
	if err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.stdout, "recorded PR #%d: %s\n", req.pr, path)
	return 0
}
