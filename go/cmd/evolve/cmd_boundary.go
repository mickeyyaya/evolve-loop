package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

const (
	boundaryUsage   = "usage: evolve boundary run [--merge n,...] --goal-text-file F [--max-cycles N] [--dry-run] [--project-root P]"
	boundaryPrefix  = "evolve boundary run: "
	boundaryLoopLog = "boundary-loop.log"
)

type boundaryDispatch func(verb string, args []string, stdout, stderr io.Writer) int

type boundaryStep struct {
	verb string
	args []string
	desc string
}

func (s boundaryStep) String() string {
	if s.desc != "" {
		return s.desc
	}
	words := []string{"evolve", s.verb}
	for _, a := range s.args {
		if a == "" || strings.ContainsAny(a, " \t\n'\"\\") {
			a = strconv.Quote(a)
		}
		words = append(words, a)
	}
	return strings.Join(words, " ")
}

type boundaryArgs struct {
	prs                 []string
	goalText, maxCycles string
	projectRoot         string
	runID               string
	dryRun              bool
}

func runBoundary(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runBoundaryWith(dispatchBoundaryVerb, args, stdout, stderr)
}

func dispatchBoundaryVerb(verb string, args []string, stdout, stderr io.Writer) int {
	handlers := map[string]func([]string, io.Reader, io.Writer, io.Writer) int{
		"loop-stop": runLoopStop, "pr": runPR, "sync-main": runSyncMain, "gc": runGC, "loop": runLoop,
		boundaryLogVerb: runBoundaryLog,
	}
	run, ok := handlers[verb]
	if !ok {
		fmt.Fprintf(stderr, "%sno handler for step %q\n", boundaryPrefix, verb)
		return exitIO
	}
	return run(args, strings.NewReader(""), stdout, stderr)
}

func runBoundaryWith(dispatch boundaryDispatch, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintln(stderr, boundaryUsage)
		return exitUsage
	}
	a, err := parseBoundaryArgs(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", boundaryPrefix, err, boundaryUsage)
		return exitUsage
	}
	root, err := loopStopRoot(a.projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", boundaryPrefix, err)
		return exitIO
	}
	a.runID = gcpolicy.LogRunID(time.Now())
	steps := boundarySteps(a, root)
	if a.dryRun {
		fmt.Fprintln(stdout, "boundary: dry run; would run, in order:")
		for i, s := range steps {
			fmt.Fprintf(stdout, "  %d. %s\n", i+1, s)
		}
		return 0
	}
	return runBoundarySteps(dispatch, steps, stdout, stderr)
}

func parseBoundaryArgs(args []string) (boundaryArgs, error) {
	var a boundaryArgs
	var merge, goalFile string
	operands, err := cliFlags{
		bools: map[string]*bool{"--dry-run": &a.dryRun},
		values: map[string]*string{
			"--merge": &merge, "--goal-text-file": &goalFile, "--max-cycles": &a.maxCycles, "--project-root": &a.projectRoot,
		},
	}.parse(args)
	switch {
	case err != nil:
		return a, err
	case len(operands) > 0:
		return a, fmt.Errorf("unexpected argument %q", operands[0])
	case goalFile == "":
		return a, errors.New("--goal-text-file is required")
	}
	if a.maxCycles != "" {
		if n, err := strconv.Atoi(a.maxCycles); err != nil || n <= 0 {
			return a, fmt.Errorf("--max-cycles %q must be a positive number", a.maxCycles)
		}
	}
	if a.prs, err = boundaryPRs(merge); err != nil {
		return a, err
	}
	goal, err := os.ReadFile(goalFile)
	if err != nil {
		return a, fmt.Errorf("--goal-text-file: %w", err)
	}
	if a.goalText = strings.TrimSpace(string(goal)); a.goalText == "" {
		return a, fmt.Errorf("--goal-text-file %s is empty", goalFile)
	}
	return a, nil
}

func boundaryPRs(merge string) ([]string, error) {
	if merge == "" {
		return nil, nil
	}
	prs := strings.Split(merge, ",")
	seen := map[string]bool{}
	for _, n := range prs {
		if !prNumber.MatchString(n) || seen[n] {
			return nil, fmt.Errorf("--merge %q: %q is not a distinct positive PR number", merge, n)
		}
		seen[n] = true
	}
	return prs, nil
}

func boundarySteps(a boundaryArgs, root string) []boundaryStep {
	return append(boundaryHaltSteps(a, root), boundaryLaunchSteps(a, root)...)
}

func boundaryHaltSteps(a boundaryArgs, root string) []boundaryStep {
	at := []string{"--project-root", root}
	steps := []boundaryStep{{verb: "loop-stop", args: append([]string{"--wait"}, at...)}}
	if len(a.prs) > 0 {
		steps = append(steps, boundaryStep{verb: "pr", args: append(append([]string{"merge"}, a.prs...), at...)})
	}
	return append(steps, boundaryStep{verb: "sync-main", args: at}, boundaryStep{verb: "gc", args: at})
}

func boundaryLoopLogPath(root, runID string) string {
	return filepath.Join(root, ".evolve", gcpolicy.LogsDir, runID, gcpolicy.LoopLogName)
}

func boundaryLaunchSteps(a boundaryArgs, root string) []boundaryStep {
	at := []string{"--project-root", root}
	launch := []string{"--detach", "--log", boundaryLoopLogPath(root, a.runID), "--goal-text", a.goalText}
	if a.maxCycles != "" {
		launch = append(launch, "--max-cycles", a.maxCycles)
	}
	return []boundaryStep{
		{verb: "loop-stop", args: append([]string{"--release"}, at...)},
		{verb: boundaryLogVerb, args: append([]string{"--run-id", a.runID}, at...), desc: boundaryLogStepDesc(a.runID)},
		{verb: "loop", args: append(launch, at...)},
	}
}

func runBoundarySteps(dispatch boundaryDispatch, steps []boundaryStep, stdout, stderr io.Writer) int {
	released := false
	for i, s := range steps {
		fmt.Fprintf(stdout, "boundary: step %d/%d: %s\n", i+1, len(steps), s)
		if rc := dispatch(s.verb, s.args, stdout, stderr); rc != 0 {
			brake := "the loop-stop brake stays engaged"
			if released {
				brake = "the brake was already released"
			}
			fmt.Fprintf(stderr, "%sstep %d/%d (%s) failed rc=%d; stopped there, %s\n", boundaryPrefix, i+1, len(steps), s, rc, brake)
			return rc
		}
		released = released || slices.Contains(s.args, "--release")
	}
	return 0
}
