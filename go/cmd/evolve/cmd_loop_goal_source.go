package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

const lastGoalFile = "last-goal.txt"

func lastGoalPath(evolveDir string) string {
	return filepath.Join(evolveDir, lastGoalFile)
}

func countGoalSources(lf *loopArgFlags, posGoal string) int {
	n := 0
	for _, set := range []bool{lf.goalText != "", lf.goalHash != "", lf.goalFile != "", lf.goalFromLast, posGoal != ""} {
		if set {
			n++
		}
	}
	return n
}

func goalSourceName(lf *loopArgFlags, posGoal string) string {
	switch {
	case lf.goalFile != "":
		return "goal-file"
	case lf.goalFromLast:
		return "goal-from-last"
	case lf.goalHash != "":
		return "goal-hash"
	case lf.goalText != "":
		return "goal-text"
	case posGoal != "":
		return "positional"
	default:
		return ""
	}
}

func loadGoalSourceText(stderr io.Writer, lf *loopArgFlags, posGoal string) (text string, rc int) {
	exclusive := lf.goalFile != "" || lf.goalFromLast
	if exclusive && countGoalSources(lf, posGoal) > 1 {
		fmt.Fprintln(stderr, "evolve loop: pass one goal source — --goal-file and --goal-from-last exclude --goal-text, --goal-hash, a positional goal and each other")
		return "", 10
	}
	switch {
	case lf.goalFile != "":
		return readGoalFile(stderr, lf.goalFile)
	case lf.goalFromLast:
		return readLastGoal(stderr, lf.evolveDir)
	default:
		return lf.goalText, 0
	}
}

func readGoalFile(stderr io.Writer, path string) (string, int) {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: --goal-file %q: %v\n", path, err)
		return "", 10
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		fmt.Fprintf(stderr, "evolve loop: --goal-file %q holds no goal text\n", path)
		return "", 10
	}
	return text, 0
}

func readLastGoal(stderr io.Writer, evolveDir string) (string, int) {
	path := lastGoalPath(evolveDir)
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "evolve loop: --goal-from-last: no goal recorded at %s — launch once with --goal-text or --goal-file\n", path)
		return "", 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: --goal-from-last: read %s: %v\n", path, err)
		return "", 1
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		fmt.Fprintf(stderr, "evolve loop: --goal-from-last: the record %s is empty — launch once with --goal-text or --goal-file\n", path)
		return "", 1
	}
	return text, 0
}

func recordLaunchGoal(cfg loopConfig, stderr io.Writer) {
	if cfg.DryRun || cfg.GoalText == "" {
		return
	}
	path := lastGoalPath(cfg.EvolveDir)
	if err := atomicwrite.Bytes(path, []byte(cfg.GoalText+"\n")); err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: could not record the goal for --goal-from-last at %s: %v\n", path, err)
	}
}
