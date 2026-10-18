package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
)

func refuseBoundaryWrite(root string, getenv func(string) string, now time.Time, subject string) error {
	if subagent.ReadDispatchDepth(getenv) > 0 {
		return fmt.Errorf("refused inside a phase: %s changes at a wave boundary, never from a phase agent", subject)
	}
	live := runlease.LiveRuns(filepath.Join(root, ".evolve", "runs"), now)
	if len(live) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(live))
	for _, run := range live {
		dirs = append(dirs, filepath.Base(run.Dir))
	}
	return fmt.Errorf("refused while a cycle runs (%s holds a live lease): change %s at a wave boundary", strings.Join(dirs, ", "), subject)
}
