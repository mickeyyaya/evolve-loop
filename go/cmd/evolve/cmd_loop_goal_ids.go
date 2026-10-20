package main

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

const exitGoalNamesInertItems = 10

func refuseInertGoalItems(cfg loopConfig, stderr io.Writer) int {
	if cfg.Resume || cfg.GoalText == "" {
		return 0
	}
	inert, err := inertGoalItems(cfg.ProjectRoot, cfg.EvolveDir, cfg.GoalText, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: cannot check the items the goal names: %v\n", err)
		return 0
	}
	if len(inert) == 0 {
		return 0
	}
	fmt.Fprintln(stderr, "evolve loop: the goal names inbox items that no lane can take; goal text does not select or exclude inbox items, so remove these ids from the goal or route the items:")
	for _, line := range inert {
		fmt.Fprintf(stderr, "  %s\n", line)
	}
	return exitGoalNamesInertItems
}

func inertGoalItems(projectRoot, evolveDir, goal string, warn io.Writer) ([]string, error) {
	inboxDir := filepath.Join(evolveDir, "inbox")
	scan, err := inboxbatch.ScanDir(inboxDir)
	if err != nil {
		return nil, fmt.Errorf("inbox %s: %w", inboxDir, err)
	}
	named := goalTokens(goal)
	opts := inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}
	isProtected := laneForbidden(projectRoot, warn)
	var inert []string
	for _, it := range scan.Items {
		if !named[it.ID] {
			continue
		}
		delete(named, it.ID)
		if place, reason := inboxmover.PlaceOnLaneMenu(opts, it, isProtected); place != inboxmover.MenuReady {
			inert = append(inert, it.ID+": "+reason)
		}
	}
	for id, recs := range inboxbatch.ConsumedRecords(inboxDir) {
		if named[id] {
			inert = append(inert, id+": "+recs[0].Describe())
		}
	}
	slices.Sort(inert)
	return inert, nil
}

func goalTokens(goal string) map[string]bool {
	tokens := map[string]bool{}
	for _, t := range strings.FieldsFunc(goal, func(r rune) bool { return !isIDRune(r) }) {
		if id := strings.Trim(t, "-"); strings.Contains(id, "-") {
			tokens[id] = true
		}
	}
	return tokens
}

func isIDRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_'
}
