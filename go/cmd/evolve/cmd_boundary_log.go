package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const (
	boundaryLogVerb   = "boundary-log"
	boundaryLogPrefix = "evolve boundary-log: "
	legacyLogSuffix   = "-legacy"
)

var errBoundaryLogLive = errors.New("a run is live")

func boundaryLogStepDesc(runID string) string {
	return fmt.Sprintf("link .evolve/logs/current to logs/%s and .evolve/%s to logs/current/%s", runID, boundaryLoopLog, gcpolicy.LoopLogName)
}

func runBoundaryLog(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var projectRoot, runID string
	operands, err := cliFlags{values: map[string]*string{"--project-root": &projectRoot, "--run-id": &runID}}.parse(args)
	if err == nil && (len(operands) > 0 || !gcpolicy.IsLogRunID(runID)) {
		err = fmt.Errorf("--run-id %q is not a log run id", runID)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", boundaryLogPrefix, err)
		return exitUsage
	}
	root, err := loopStopRoot(projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n", boundaryLogPrefix, err)
		return exitIO
	}
	evolveDir := filepath.Join(root, ".evolve")
	live := func() bool { return len(runlease.LiveRuns(filepath.Join(evolveDir, "runs"), time.Now())) > 0 }
	if err := prepareBoundaryLog(evolveDir, runID, live); err != nil {
		fmt.Fprintf(stderr, "%s%v\n", boundaryLogPrefix, err)
		if errors.Is(err, errBoundaryLogLive) {
			return exitRefused
		}
		return exitIO
	}
	fmt.Fprintf(stdout, "boundary: log dir %s; %s\n", filepath.Join(evolveDir, gcpolicy.LogsDir, runID), boundaryLogStepDesc(runID))
	return 0
}

func prepareBoundaryLog(evolveDir, runID string, live func() bool) error {
	logsDir := filepath.Join(evolveDir, gcpolicy.LogsDir)
	link := filepath.Join(evolveDir, boundaryLoopLog)
	if err := moveLegacyLoopLog(link, filepath.Join(logsDir, runID+legacyLogSuffix), live); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(logsDir, runID), 0o755); err != nil {
		return fmt.Errorf("make the run log dir: %w", err)
	}
	if err := replaceSymlink(runID, filepath.Join(logsDir, gcpolicy.LogsCurrentLink)); err != nil {
		return err
	}
	return replaceSymlink(filepath.Join(gcpolicy.LogsDir, gcpolicy.LogsCurrentLink, gcpolicy.LoopLogName), link)
}

func moveLegacyLoopLog(link, legacyDir string, live func() bool) error {
	info, err := os.Lstat(link)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	if live() {
		return fmt.Errorf("%w: it can write %s; stop the loop first", errBoundaryLogLive, link)
	}
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		return fmt.Errorf("make the legacy log dir: %w", err)
	}
	dst := filepath.Join(legacyDir, gcpolicy.LoopLogName)
	if err := os.Link(link, dst); err != nil && !sameFile(link, dst) {
		return fmt.Errorf("move the legacy loop log to %s without overwrite: %w", dst, err)
	}
	if err := os.Remove(link); err != nil {
		return fmt.Errorf("remove the legacy loop log after its copy to %s: %w", dst, err)
	}
	return nil
}

func replaceSymlink(target, path string) error {
	tmp := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear %s: %w", tmp, err)
	}
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("link %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("switch %s: %w", path, err)
	}
	return nil
}

func sameFile(a, b string) bool {
	ai, aerr := os.Lstat(a)
	bi, berr := os.Lstat(b)
	return aerr == nil && berr == nil && os.SameFile(ai, bi)
}
