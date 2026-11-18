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

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	integrationBatchUsage  = "usage: evolve integration batch [--dry-run] [--main-red-fix] [--project-root P] [PR...]"
	integrationRecordUsage = "usage: evolve integration record-pr --pr N (--reviewed | --class NAME)"
	inboxDir               = ".evolve/inbox/"
	mainRedFixFlag         = "--main-red-fix"
)

var reviewClasses = map[string]func(paths []string) bool{
	"docs-review": docsOnlyData,
}

type batchPorts struct {
	Repo integration.Repo
	CI   integration.GhCI
	Main integration.GitMain
}

func (e integrationEnv) ports() batchPorts {
	repo := integration.Repo{Dir: e.Root, Git: e.Git}
	return batchPorts{Repo: repo, CI: integration.GhCI{Repo: repo, Gh: e.Gh, Settle: e.Settle}, Main: integration.GitMain{Repo: repo}}
}

func (e integrationEnv) train(projectRoot string, warn io.Writer) integration.Train {
	p := e.ports()
	gate := e.stopGate(projectRoot, warn)
	return integration.Train{
		Composer: integration.Composer{Repo: p.Repo, Stage: policy.FleetIntegrationShadow, Regen: e.regenPort(p.Repo, projectRoot, warn)},
		L2:       e.batchTest(p.Repo, projectRoot, warn),
		CI:       gate.CI(p.CI),
		Main:     gate.Main(p.Main),
		Host:     integration.GhHost{CI: p.CI},
		Inbox:    inboxCloser(e.Root),
		Root:     e.Root,
		Wave:     e.Wave,
		MainCI:   p.CI,
	}
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
		fmt.Fprintf(c.stderr, "Next: rerun the failed job of main when the red is a flake, or land the fix alone with %s <PR>\n", mainRedFixFlag)
	}
	return refusalExit(err)
}

func usageFail(stderr io.Writer, err error, usage string) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", integrationPrefix, err, usage)
	return exitUsage
}

func batchVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var dry, help, redFix bool
	var rootFlag string
	operands, err := cliFlags{bools: map[string]*bool{"--dry-run": &dry, "--help": &help, mainRedFixFlag: &redFix}, values: map[string]*string{"--project-root": &rootFlag}}.parse(args)
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
	t := env.train(root, stderr)
	t.MainRedFix, t.Progress = redFix, stepPrinter(stdout)
	if dry {
		t.MainCI = integration.PeekCI{CI: env.ports().CI}
	}
	return runBatch(t, batchRequest{prs: prs, dry: dry}, cliIO{stdout, stderr})
}

func stepPrinter(w io.Writer) func(integration.Step) {
	return func(s integration.Step) {
		fmt.Fprintln(w, stepLine(s))
	}
}

func stepLine(s integration.Step) string {
	prs := joinInts(s.PRs)
	switch s.Kind {
	case integration.StepTest:
		return fmt.Sprintf("step: test PRs %s in batch PR #%d", prs, s.Batch)
	case integration.StepVerdict:
		return fmt.Sprintf("step: batch PR #%d of PRs %s is %s", s.Batch, prs, s.Verdict)
	case integration.StepRerun:
		return fmt.Sprintf("step: rerun the failed jobs of batch PR #%d", s.Batch)
	case integration.StepSplit:
		return fmt.Sprintf("step: split PRs %s in halves", prs)
	case integration.StepLand:
		return fmt.Sprintf("step: landed PRs %s through batch PR #%d", prs, s.Batch)
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
	operands, err := cliFlags{bools: map[string]*bool{"--help": &help, "--reviewed": &reviewed}, values: map[string]*string{"--pr": &pr, "--class": &class}}.parse(args)
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
	return recordPR(env.train(env.Root, stderr), recordRequest{pr: n, bind: bind}, c)
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
