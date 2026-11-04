package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	integrationBatchUsage  = "usage: evolve integration batch [--dry-run] [PR...]"
	integrationRecordUsage = "usage: evolve integration record-pr --pr N (--attestation FILE | --class NAME)"
	inboxDir               = ".evolve/inbox/"
)

var reviewClasses = map[string]func(paths []string) bool{
	"docs-review": docsOnlyData,
}

func (e integrationEnv) train() integration.Train {
	repo := integration.Repo{Dir: e.Root, Git: e.Git}
	ci := integration.GhCI{Repo: repo, Gh: e.Gh, Settle: e.Settle}
	return integration.Train{
		Composer: integration.Composer{Repo: repo, Stage: policy.FleetIntegrationShadow},
		CI:       ci,
		Main:     integration.GitMain{Repo: repo},
		Host:     integration.GhHost{CI: ci},
		Inbox:    inboxCloser(e.Root),
		Root:     e.Root,
		Wave:     e.Wave,
	}
}

func inboxCloser(root string) integration.InboxFunc {
	return func(_ context.Context, ids []string) ([]string, error) {
		return inboxmover.ReconcileSuperseded(inboxmover.Options{ProjectRoot: root}, ids, "processed", inboxmover.PromoteOpts{})
	}
}

type cliIO struct {
	stdout io.Writer
	stderr io.Writer
}

func (c cliIO) fail(err error) int {
	fmt.Fprintf(c.stderr, "%s%v\n", integrationPrefix, err)
	return refusalExit(err)
}

func usageFail(stderr io.Writer, err error, usage string) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", integrationPrefix, err, usage)
	return exitUsage
}

func batchVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var dry, help bool
	operands, err := cliFlags{bools: map[string]*bool{"--dry-run": &dry, "--help": &help}}.parse(args)
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
	if err != nil {
		return usageFail(stderr, err, integrationBatchUsage)
	}
	return runBatch(env.train(), batchRequest{prs: prs, dry: dry}, cliIO{stdout, stderr})
}

type batchRequest struct {
	prs []int
	dry bool
}

func runBatch(t integration.Train, req batchRequest, c cliIO) int {
	ctx := context.Background()
	prs := req.prs
	if len(prs) == 0 {
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
	if errors.As(err, &refusal) {
		return exitRefused
	}
	return exitIO
}

func printPlan(w io.Writer, p integration.Plan) {
	fmt.Fprintf(w, "order: %s\n", joinInts(p.Order))
	for _, e := range p.Ejected {
		fmt.Fprintf(w, "eject %s %s %s\n", e.Label(), e.Cause, strings.Join(e.Paths, " "))
	}
	fmt.Fprintf(w, "batch: %d PRs\n", len(p.Kept))
}

func joinInts(items []int) string {
	parts := make([]string, len(items))
	for i, n := range items {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, " ")
}

func recordPRVerb(env integrationEnv, args []string, stdout, stderr io.Writer) int {
	var help bool
	var pr, attestation, class string
	operands, err := cliFlags{bools: map[string]*bool{"--help": &help}, values: map[string]*string{"--pr": &pr, "--attestation": &attestation, "--class": &class}}.parse(args)
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
	case (attestation == "") == (class == ""):
		err = errors.New("name exactly one of --attestation and --class")
	}
	if err != nil {
		return usageFail(stderr, err, integrationRecordUsage)
	}
	return recordPR(env.train(), recordRequest{pr: n, bind: binderFor(env.Root, attestation, class)}, cliIO{stdout, stderr})
}

func binderFor(root, attestation, class string) integration.Binder {
	if attestation == "" {
		return classBinder(class)
	}
	return attestedBinder(root, attestation)
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

func attestedBinder(root, path string) integration.Binder {
	return func(ev integration.Evidence) (integration.AuditBinding, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return integration.AuditBinding{}, fmt.Errorf("read the attestation: %w", err)
		}
		var att commitgate.Attestation
		if err := json.Unmarshal(raw, &att); err != nil {
			return integration.AuditBinding{}, fmt.Errorf("parse the attestation: %w", err)
		}
		if att.ReviewWaiver != "" {
			return integration.AuditBinding{}, errors.New("a waiver is not a review")
		}
		if err := commitgate.VerifyAttestedRuns(root, att); err != nil {
			return integration.AuditBinding{}, err
		}
		return integration.AttestedBinding(ev, att.TreeStateSHA, integration.Digest(raw))
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
