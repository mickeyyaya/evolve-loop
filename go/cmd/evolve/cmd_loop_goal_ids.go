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

type goalItems struct {
	inert      []string
	consumed   []string
	selectable []string
}

func relaunchArgv(argv []string) []string {
	if len(argv) < 2 || argv[1] != "loop" || slices.Contains(argv, "--relaunch") {
		return argv
	}
	return slices.Concat(argv[:2], []string{"--relaunch"}, argv[2:])
}

func operatorGoalLaunch(cfg loopConfig) bool {
	return cfg.GoalText != "" && !cfg.Resume && !cfg.HandedOff && !cfg.Relaunch && cfg.GoalSource != "goal-from-last"
}

func refuseInertGoalItems(cfg loopConfig, stderr io.Writer) int {
	if !operatorGoalLaunch(cfg) {
		return 0
	}
	items, err := namedGoalItems(cfg.ProjectRoot, cfg.EvolveDir, cfg.GoalText, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: cannot check the items the goal names: %v\n", err)
		return 0
	}
	warnGoalItems(items, stderr)
	if len(items.inert) == 0 {
		return 0
	}
	fmt.Fprintln(stderr, "evolve loop: GOAL_NAMES_INERT_ITEMS: the goal names inbox items that no lane can take; goal text does not select or exclude inbox items, so remove these ids from the goal or route the items:")
	for _, line := range items.inert {
		fmt.Fprintf(stderr, "  %s\n", line)
	}
	return exitGoalNamesInertItems
}

func warnGoalItems(items goalItems, stderr io.Writer) {
	for _, line := range items.consumed {
		fmt.Fprintf(stderr, "evolve loop: WARN: the goal names a consumed item, which no lane takes: %s\n", line)
	}
	if len(items.selectable) > 0 {
		fmt.Fprintf(stderr, "evolve loop: WARN: goal text does not select or exclude inbox items; the lanes can take %s whatever the goal says. To keep the lanes off an item, run `evolve inbox route-console <id> <reason> <cycle>`\n", strings.Join(items.selectable, ", "))
	}
}

func namedGoalItems(projectRoot, evolveDir, goal string, warn io.Writer) (goalItems, error) {
	inboxDir := filepath.Join(evolveDir, "inbox")
	scan, err := inboxbatch.ScanDir(inboxDir)
	if err != nil {
		return goalItems{}, fmt.Errorf("inbox %s: %w", inboxDir, err)
	}
	named := goalTokens(goal)
	opts := inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}
	isProtected := laneForbidden(projectRoot, warn)
	var items goalItems
	for _, it := range scan.Items {
		if !named[it.ID] {
			continue
		}
		delete(named, it.ID)
		if place, reason := inboxmover.PlaceOnLaneMenu(opts, it, isProtected); place != inboxmover.MenuReady {
			items.inert = append(items.inert, it.ID+": "+reason)
		} else {
			items.selectable = append(items.selectable, it.ID)
		}
	}
	for id, recs := range inboxbatch.ConsumedRecords(inboxDir) {
		if named[id] {
			items.consumed = append(items.consumed, id+": "+recs[0].Describe())
		}
	}
	slices.Sort(items.inert)
	slices.Sort(items.consumed)
	slices.Sort(items.selectable)
	return items, nil
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
