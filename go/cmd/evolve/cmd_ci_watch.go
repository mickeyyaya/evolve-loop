package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	ciWatchUsage = "usage: evolve ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N] [--project-root P]\n" +
		"  --sha S        watch a pushed commit; red files one fix-forward inbox item\n" +
		"  --pr N         watch a PR's head commit; red files no inbox item\n" +
		"  --tag T        watch a release tag: required.yml and release.yml\n" +
		"  --workflow W   watch workflow W instead of the defaults (repeatable)\n" +
		"  --cycle N      cycle number recorded in the fix-forward item\n" +
		"  exit 0 = every watched workflow green, 1 = a workflow red, 2 = runs not observable, 10 = usage"
	ciWatchPrefix   = "evolve ci watch: "
	releaseWorkflow = "release.yml"
)

type ciWatchArgs struct {
	sha, pr, tag string
	workflows    []string
	cycle        int
	projectRoot  string
	help         bool
}

func parseCIWatchArgs(args []string) (ciWatchArgs, error) {
	var a ciWatchArgs
	var cycle string
	operands, err := cliFlags{
		bools:  map[string]*bool{"--help": &a.help, "-h": &a.help},
		values: map[string]*string{"--sha": &a.sha, "--pr": &a.pr, "--tag": &a.tag, "--cycle": &cycle, "--project-root": &a.projectRoot},
		lists:  map[string]*[]string{"--workflow": &a.workflows},
	}.parse(args)
	if err != nil || a.help {
		return a, err
	}
	if len(operands) > 0 {
		return a, fmt.Errorf("unexpected operand %q", operands[0])
	}
	if err := a.validateTarget(); err != nil {
		return a, err
	}
	if cycle != "" {
		if a.cycle, err = strconv.Atoi(cycle); err != nil || a.cycle < 0 {
			return a, fmt.Errorf("--cycle %q must be a non-negative integer", cycle)
		}
	}
	for _, w := range a.workflows {
		if strings.TrimSpace(w) == "" {
			return a, errors.New("--workflow needs a workflow file name")
		}
	}
	return a, nil
}

func (a ciWatchArgs) validateTarget() error {
	set := 0
	for _, v := range []string{a.sha, a.pr, a.tag} {
		if v != "" {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("name exactly one of --sha, --pr or --tag, got %d", set)
	}
	if a.sha != "" {
		if t, err := ciwatch.ParseTarget("sha:" + a.sha); err != nil || t.Kind != ciwatch.TargetSHA {
			return fmt.Errorf("--sha %q must be a 7-40 character hex commit SHA", a.sha)
		}
	}
	if a.pr != "" {
		if _, err := ciwatch.ParseTarget("pr:" + a.pr); err != nil {
			return fmt.Errorf("--pr %q must be a positive PR number", a.pr)
		}
	}
	if strings.HasPrefix(a.tag, "-") {
		return fmt.Errorf("--tag %q is not a tag name", a.tag)
	}
	return nil
}

func (a ciWatchArgs) watchedWorkflows() []string {
	switch {
	case len(a.workflows) > 0:
		return a.workflows
	case a.tag != "":
		return []string{ciparity.RequiredWorkflow, releaseWorkflow}
	}
	return []string{ciparity.RequiredWorkflow}
}

func runCIWatch(args []string, stdout, stderr io.Writer) int {
	a, err := parseCIWatchArgs(args)
	if a.help && err == nil {
		fmt.Fprintln(stdout, ciWatchUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", ciWatchPrefix, err, ciWatchUsage)
		return exitUsage
	}
	root, err := loopStopRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciWatchPrefix, err)
		return exitIO
	}
	ctx := context.Background()
	opts, err := ciWatchOptions(ctx, root, a)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciWatchPrefix, err)
		return exitIO
	}
	red, err := watchWorkflows(ctx, root, a.watchedWorkflows(), opts, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", ciWatchPrefix, err)
		return exitIO
	}
	if len(red) == 0 {
		return 0
	}
	classifyRedRuns(ctx, root, red, stdout, stderr)
	return exitRefused
}

func ciWatchOptions(ctx context.Context, root string, a ciWatchArgs) (ciwatch.Options, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return ciwatch.Options{}, err
	}
	cw, err := pol.CIWatchConfig()
	if err != nil {
		return ciwatch.Options{}, err
	}
	sha, err := ciWatchHead(ctx, root, a)
	if err != nil {
		return ciwatch.Options{}, err
	}
	return ciwatch.Options{
		SHA: sha, Cycle: a.cycle, InboxDir: filepath.Join(paths.EvolveDirOf(root), "inbox"), NoEscalation: a.pr != "",
		Timeout: time.Duration(*cw.TimeoutS) * time.Second, Poll: time.Duration(*cw.PollS) * time.Second,
	}, nil
}

func ciWatchHead(ctx context.Context, root string, a ciWatchArgs) (string, error) {
	switch {
	case a.pr != "":
		return ciwatch.ResolvePRHead(ctx, root, a.pr)
	case a.tag != "":
		return ciwatch.ResolveCommit(ctx, root, a.tag)
	}
	return ciwatch.ResolveCommit(ctx, root, a.sha)
}

type redWorkflow struct {
	workflow string
	runID    int64
}

func watchWorkflows(ctx context.Context, root string, workflows []string, opts ciwatch.Options, stdout io.Writer) ([]redWorkflow, error) {
	var red []redWorkflow
	for _, w := range workflows {
		var runID int64
		fetch := ciwatch.NewGHWorkflowFetcher(root, w)
		opts.Fetch = func(ctx context.Context, sha string) (ciwatch.RunStatus, error) {
			st, err := fetch(ctx, sha)
			runID = st.RunID
			return st, err
		}
		rec, err := ciwatch.Watch(ctx, opts)
		if err != nil {
			return red, fmt.Errorf("%s: %w", w, err)
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", w, rec.Conclusion, shortSHA(rec.SHA), rec.RunURL)
		if rec.Conclusion != ciwatch.ConclusionSuccess {
			red = append(red, redWorkflow{workflow: w, runID: runID})
		}
	}
	return red, nil
}

func classifyRedRuns(ctx context.Context, root string, red []redWorkflow, stdout, stderr io.Writer) {
	opts, err := ciClassifyOptions(root, false, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%sclassify: %v\n", ciWatchPrefix, err)
		return
	}
	for _, r := range red {
		target := ciwatch.Target{Kind: ciwatch.TargetRun, Value: strconv.FormatInt(r.runID, 10)}
		rep, err := ciwatch.ClassifyTarget(ctx, ciwatch.NewGHClassifySource(root), target, opts)
		if err == nil {
			err = writeClassifyReport(stdout, rep, false)
		}
		if err != nil {
			fmt.Fprintf(stderr, "%sclassify %s run %d: %v\n", ciWatchPrefix, r.workflow, r.runID, err)
		}
	}
}
