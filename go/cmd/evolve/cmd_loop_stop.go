package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func runLoopStop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve loop-stop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	var release bool
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.BoolVar(&release, "release", false, "remove the brake so the next evolve loop launch runs")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "evolve loop-stop: unexpected argument %q (usage: evolve loop-stop [--release] [--project-root P])\n", fs.Arg(0))
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
	return engageLoopStop(brake, time.Now(), stdout, stderr)
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
