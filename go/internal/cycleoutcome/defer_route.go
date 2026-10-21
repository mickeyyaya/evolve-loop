package cycleoutcome

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var (
	depsEdit     = inboxmover.EditHeld
	consoleRoute = inboxmover.RouteConsole
)

const companionWeight = 0.8

func routeDeferral(opts inboxmover.Options, in FailureInputs, committed []string, stderr io.Writer) []string {
	d := in.Defer
	if d == (core.RecordedDefer{}) || len(committed) == 0 {
		return nil
	}
	reason := fmt.Sprintf("cycle %d ended by a defer-or-quarantine decision: category %s, fix_type %s", in.Cycle, d.Category, d.FixType)
	switch d.FixType {
	case policy.FixTypeReintent:
		return routeEachConsole(opts, committed, reason, in.Cycle, stderr)
	case policy.FixTypePipelineRepair:
		id, err := fileCompanion(opts, in.Cycle, reason, committed)
		if err != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: the companion item %s was not filed and stamped (%v); the items are released\n", id, err)
			return nil
		}
		return waitOnCompanion(opts, committed, id, in.Cycle, stderr)
	default:
		fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: fix_type %q has no route; the items are released\n", d.FixType)
		return nil
	}
}

func routeEachConsole(opts inboxmover.Options, ids []string, reason string, cycle int, stderr io.Writer) []string {
	var routed []string
	for _, id := range ids {
		if _, err := consoleRoute(opts, id, reason, cycle); err != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: %s was not routed to the console (%v); it is released with a failure bump\n", id, err)
			continue
		}
		routed = append(routed, id)
	}
	return routed
}

func fileCompanion(opts inboxmover.Options, cycle int, reason string, waiting []string) (string, error) {
	id := "pipeline-repair-cycle-" + strconv.Itoa(cycle)
	if inboxmover.ResolveDispatchState(opts, id).State != inboxmover.StateUnknown {
		return id, nil
	}
	raw, _ := json.Marshal(map[string]any{
		"id":             id,
		"title":          fmt.Sprintf("Repair the pipeline defect that ended cycle %d", cycle),
		"kind":           "fix",
		"summary":        reason + ". The items " + strings.Join(waiting, ", ") + " wait on this item.",
		"fix":            "Read the failure-decision.json and the retrospective of cycle " + strconv.Itoa(cycle) + ", repair the pipeline, then consume this item so the waiting items can run.",
		"acceptance":     []string{"The defect that the retrospective of cycle " + strconv.Itoa(cycle) + " classified is repaired, and a test pins the repair"},
		"route":          inboxmover.RouteConsoleValue,
		"priority_class": "stability",
		"weight":         companionWeight,
	})
	if _, err := inboxmover.File(opts, raw); err != nil {
		return id, err
	}
	_, err := inboxmover.RouteConsole(opts, id, reason, cycle)
	return id, err
}

func waitOnCompanion(opts inboxmover.Options, ids []string, companion string, cycle int, stderr io.Writer) []string {
	var waiting []string
	for _, id := range ids {
		if hasDep(opts, id, companion) {
			waiting = append(waiting, id)
			continue
		}
		if _, err := depsEdit(opts, id, cycle, []inboxmover.FieldEdit{{Op: inboxmover.EditAdd, Field: "deps", Value: companion}}); err != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: %s did not gain deps %s (%v); it is released with a failure bump\n", id, companion, err)
			continue
		}
		waiting = append(waiting, id)
	}
	return waiting
}

func hasDep(opts inboxmover.Options, id, dep string) bool {
	item, _, err := inboxbatch.LoadFile(inboxmover.ResolveDispatchState(opts, id).Path)
	return err == nil && slices.Contains(item.Deps, dep)
}
