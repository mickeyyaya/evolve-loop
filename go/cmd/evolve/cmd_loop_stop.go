package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	loopStopDefaultWait = 10 * time.Minute
	loopStopWaitPoll    = 200 * time.Millisecond
	loopStopUsage       = "usage: evolve loop-stop [--release | --wait [--timeout D]] [--project-root P]"
)

func runLoopStop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve loop-stop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var release, wait bool
	var timeout time.Duration
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&release, "release", false, "remove the brake so the next evolve loop launch runs")
	fs.BoolVar(&wait, "wait", false, "after engaging the brake, block until no run lease is live")
	fs.DurationVar(&timeout, "timeout", loopStopDefaultWait, "with --wait: give up after this long (exit 1, brake kept)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve loop-stop: unexpected argument %q (%s)\n", fs.Arg(0), loopStopUsage)
		return 1
	}
	timeoutSet := false
	fs.Visit(func(f *flag.Flag) { timeoutSet = timeoutSet || f.Name == "timeout" })
	switch {
	case wait && release:
		fmt.Fprintf(stderr, "evolve loop-stop: --wait and --release are mutually exclusive (%s)\n", loopStopUsage)
		return 10
	case timeoutSet && !wait:
		fmt.Fprintf(stderr, "evolve loop-stop: --timeout requires --wait (%s)\n", loopStopUsage)
		return 1
	case wait && timeout <= 0:
		fmt.Fprintf(stderr, "evolve loop-stop: --timeout must be positive, got %s\n", timeout)
		return 1
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop-stop: cwd: %v\n", err)
		return 1
	}
	brake := paths.LoopStopPath(paths.EvolveDirOf(root))
	if release {
		return releaseLoopStop(brake, stdout, stderr)
	}
	if rc := engageLoopStop(brake, time.Now(), stdout, stderr); rc != 0 || !wait {
		return rc
	}
	runsDir := filepath.Join(paths.EvolveDirOf(root), "runs")
	return waitForIdleLoop(runsDir, timeout, loopStopWaitPoll, time.Now, time.Sleep, stdout, stderr)
}

func waitForIdleLoop(runsDir string, timeout, poll time.Duration, now func() time.Time, sleep func(time.Duration), stdout, stderr io.Writer) int {
	deadline := now().Add(timeout)
	shown := map[string]string{}
	for {
		live := runlease.LiveRuns(runsDir, now())
		if len(live) == 0 {
			fmt.Fprintln(stdout, "loop-stop: no run lease is live; the loop is idle")
			return 0
		}
		for _, r := range live {
			reportLiveRunPhase(stdout, r, shown)
		}
		if !now().Before(deadline) {
			for _, r := range live {
				fmt.Fprintf(stderr, "evolve loop-stop: timed out after %s: run %s still live (%s); the brake stays engaged\n", timeout, r.Lease.RunID, r.Dir)
			}
			return 1
		}
		sleep(poll)
	}
}

func reportLiveRunPhase(w io.Writer, r runlease.LiveRun, shown map[string]string) {
	phase := liveRunPhase(r.Dir)
	if _, seen := shown[r.Dir]; phase == "" && seen {
		return
	}
	if phase == "" {
		phase = "unknown"
	}
	key := r.Lease.RunID + "/" + phase
	if shown[r.Dir] == key {
		return
	}
	shown[r.Dir] = key
	fmt.Fprintf(w, "loop-stop: waiting on run %s (%s) in phase %s\n", r.Lease.RunID, filepath.Base(r.Dir), phase)
}

func liveRunPhase(runDir string) string {
	b, err := os.ReadFile(filepath.Join(runDir, "run.json"))
	if err != nil {
		return ""
	}
	var run struct {
		Phase string `json:"phase"`
	}
	if json.Unmarshal(b, &run) != nil {
		return ""
	}
	return run.Phase
}

func loopStopRoot(projectRoot string, stderr io.Writer) (string, error) {
	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		projectRoot = cwd
	}
	return paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve loop-stop: WARN: %s\n", m)
	}), nil
}

func engageLoopStop(brake string, now time.Time, stdout, stderr io.Writer) int {
	if err := os.WriteFile(brake, []byte(now.UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		fmt.Fprintf(stderr, "evolve loop-stop: engage the brake: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "loop-stop: brake engaged at %s\n", brake)
	fmt.Fprintf(stdout, "loop-stop:   a running evolve loop finishes its current wave, closes it out, and exits at the next wave boundary (stop_reason %s); a chained run also stops at its batch boundary.\n", loopOperatorBrakeStop)
	fmt.Fprintln(stdout, "loop-stop:   until it is released, a new evolve loop launch stops before its first wave; release it with: evolve loop-stop --release")
	return 0
}

func releaseLoopStop(brake string, stdout, stderr io.Writer) int {
	err := os.Remove(brake)
	switch {
	case errors.Is(err, os.ErrNotExist):
		fmt.Fprintf(stdout, "loop-stop: no brake at %s; nothing to release\n", brake)
	case err != nil:
		fmt.Fprintf(stderr, "evolve loop-stop: release the brake: %v\n", err)
		return 1
	default:
		fmt.Fprintf(stdout, "loop-stop: brake released (%s removed); the next evolve loop launch runs\n", brake)
	}
	return 0
}
