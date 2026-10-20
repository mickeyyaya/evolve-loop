package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/mergewindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	prWindowUsage   = "usage: evolve pr window <n> | --files-from FILE [--json] [--project-root P]"
	prWindowPrefix  = "evolve pr window: "
	exitBoundary    = 3
	prWindowTimeout = 2 * time.Minute
	reasonStageOff  = "merge windows wait for P1"
	reasonNoPaths   = "the PR changes no path"
	reasonWindow    = "every path is docs, inbox or CI-only"
)

var requiredWorkflowPath = filepath.Join(".github", "workflows", "required.yml")

type prWindowArgs struct {
	pr, filesFrom, projectRoot string
	asJSON                     bool
}

type prWindowDecision struct {
	PR            string            `json:"pr,omitempty"`
	Stage         string            `json:"stage"`
	Verdict       mergewindow.Class `json:"verdict"`
	Rule          mergewindow.Class `json:"rule"`
	Reason        string            `json:"reason"`
	BoundaryPaths []string          `json:"boundary_paths"`
}

func parsePRWindowArgs(args []string) (prWindowArgs, error) {
	var a prWindowArgs
	operands, err := cliFlags{
		bools:  map[string]*bool{"--json": &a.asJSON},
		values: map[string]*string{"--files-from": &a.filesFrom, "--project-root": &a.projectRoot},
	}.parse(args)
	if err != nil {
		return a, err
	}
	switch {
	case len(operands) > 1:
		return a, errors.New("name one PR number")
	case len(operands) == 1 && a.filesFrom != "":
		return a, errors.New("name a PR number or --files-from, not both")
	case len(operands) == 0 && a.filesFrom == "":
		return a, errors.New("name a PR number or --files-from")
	case len(operands) == 1 && !prNumber.MatchString(operands[0]):
		return a, fmt.Errorf("PR operand %q must be a positive number", operands[0])
	case len(operands) == 1:
		a.pr = operands[0]
	}
	return a, nil
}

func runPRWindow(args []string, stdout, stderr io.Writer) int {
	a, err := parsePRWindowArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", prWindowPrefix, err, prWindowUsage)
		return exitUsage
	}
	root, err := loopStopRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prWindowPrefix, err)
		return exitIO
	}
	ctx, cancel := context.WithTimeout(context.Background(), prWindowTimeout)
	defer cancel()
	d, err := decidePRWindow(ctx, root, a, ghIn(root))
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prWindowPrefix, err)
		return exitIO
	}
	if err := writePRWindow(stdout, d, a.asJSON); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", prWindowPrefix, err)
		return exitIO
	}
	if d.Verdict == mergewindow.Window {
		return 0
	}
	return exitBoundary
}

func decidePRWindow(ctx context.Context, root string, a prWindowArgs, gh func(context.Context, ...string) ([]byte, error)) (prWindowDecision, error) {
	pol, err := policy.Load(paths.PolicyPath(paths.EvolveDirOf(root)))
	if err != nil {
		return prWindowDecision{}, err
	}
	stage, err := pol.PRMergeWindowStage()
	if err != nil {
		return prWindowDecision{}, err
	}
	workflow, err := os.ReadFile(filepath.Join(root, requiredWorkflowPath))
	if err != nil {
		return prWindowDecision{}, err
	}
	rule, err := mergewindow.Parse(workflow)
	if err != nil {
		return prWindowDecision{}, err
	}
	changed, err := prWindowPaths(ctx, a, gh)
	if err != nil {
		return prWindowDecision{}, err
	}
	return windowDecision(a.pr, stage, rule.Classify(changed), len(changed) == 0), nil
}

func prWindowPaths(ctx context.Context, a prWindowArgs, gh func(context.Context, ...string) ([]byte, error)) ([]string, error) {
	if a.filesFrom != "" {
		body, err := os.ReadFile(a.filesFrom)
		if err != nil {
			return nil, err
		}
		return pathLines(body), nil
	}
	out, err := gh(ctx, "api", "--paginate", "repos/{owner}/{repo}/pulls/"+a.pr+"/files", "--jq", ".[] | .filename, (.previous_filename // empty) | @json")
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, line := range pathLines(out) {
		var name string
		if err := json.Unmarshal([]byte(line), &name); err != nil {
			return nil, fmt.Errorf("gh pulls files: decode %q: %w", line, err)
		}
		changed = append(changed, name)
	}
	return changed, nil
}

func pathLines(out []byte) []string {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSuffix(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func windowDecision(pr, stage string, v mergewindow.Verdict, noPaths bool) prWindowDecision {
	d := prWindowDecision{PR: pr, Stage: stage, Verdict: v.Class, Rule: v.Class, BoundaryPaths: v.BoundaryPaths}
	if d.BoundaryPaths == nil {
		d.BoundaryPaths = []string{}
	}
	switch {
	case noPaths:
		d.Reason = reasonNoPaths
	case v.Class == mergewindow.Window:
		d.Reason = reasonWindow
	default:
		d.Reason = fmt.Sprintf("%d paths need a boundary: %s", len(v.BoundaryPaths), strings.Join(v.BoundaryPaths, ", "))
	}
	if stage != policy.MergeWindowOn {
		d.Verdict, d.Reason = mergewindow.Boundary, reasonStageOff
	}
	return d
}

func writePRWindow(w io.Writer, d prWindowDecision, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(d)
	}
	_, err := fmt.Fprintf(w, "%s: %s (stage %s, paths %s)\n", d.Verdict, d.Reason, d.Stage, d.Rule)
	return err
}
