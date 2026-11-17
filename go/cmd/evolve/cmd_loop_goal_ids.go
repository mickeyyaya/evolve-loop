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
	absent     []string
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

func refuseUnusableLaunch(cfg loopConfig, stderr io.Writer) int {
	if rc := refuseSelector(cfg, stderr); rc != 0 {
		return rc
	}
	return refuseInertGoalItems(cfg, stderr)
}

func refuseSelector(cfg loopConfig, stderr io.Writer) int {
	if both := idsInBoth(cfg.Only, cfg.Exclude); len(both) > 0 {
		fmt.Fprintf(stderr, "evolve loop: --only and --exclude both name %s; an id is selected or excluded, never both\n", strings.Join(both, ", "))
		return exitGoalNamesInertItems
	}
	if len(cfg.Only) == 0 || cfg.Resume || cfg.HandedOff || cfg.Relaunch {
		return 0
	}
	items, err := classifyInboxIDs(cfg.ProjectRoot, cfg.EvolveDir, cfg.Only, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: WARN: cannot check the items --only names: %v\n", err)
		return 0
	}
	unusable := slices.Concat(items.inert, items.consumed, items.absent)
	if len(unusable) == 0 {
		return 0
	}
	slices.Sort(unusable)
	fmt.Fprintln(stderr, "evolve loop: GOAL_NAMES_INERT_ITEMS: --only names inbox items that no lane can take; remove these ids from --only or route the items:")
	for _, line := range unusable {
		fmt.Fprintf(stderr, "  %s\n", line)
	}
	return exitGoalNamesInertItems
}

func idsInBoth(a, b []string) []string {
	var both []string
	for _, id := range a {
		if slices.Contains(b, id) && !slices.Contains(both, id) {
			both = append(both, id)
		}
	}
	slices.Sort(both)
	return both
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
	items, err := classifyNamedIDs(projectRoot, evolveDir, goalTokens(goal), warn)
	items.absent = nil
	return items, err
}

func classifyInboxIDs(projectRoot, evolveDir string, ids []string, warn io.Writer) (goalItems, error) {
	named := map[string]bool{}
	for _, id := range ids {
		named[id] = true
	}
	return classifyNamedIDs(projectRoot, evolveDir, named, warn)
}

func classifyNamedIDs(projectRoot, evolveDir string, named map[string]bool, warn io.Writer) (goalItems, error) {
	inboxDir := filepath.Join(evolveDir, "inbox")
	scan, err := inboxbatch.ScanDir(inboxDir)
	if err != nil {
		return goalItems{}, fmt.Errorf("inbox %s: %w", inboxDir, err)
	}
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
			delete(named, id)
		}
	}
	for id := range named {
		items.absent = append(items.absent, id+": not in the inbox")
	}
	slices.Sort(items.absent)
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

func warnSelectorIgnored(cfg loopConfig, stderr io.Writer) {
	if len(cfg.Only) > 0 || len(cfg.Exclude) > 0 {
		fmt.Fprintln(stderr, "[loop] WARN: --only and --exclude scope the fleet plan; this batch runs sequential, where the cycle's own triage picks the item")
	}
}
