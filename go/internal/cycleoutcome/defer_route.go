package cycleoutcome

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var depsEdit = inboxmover.Edit

const companionWeight = 0.8

type deferRoute struct {
	routed    bool
	companion string
}

func routeDeferral(opts inboxmover.Options, in FailureInputs, committed []string, stderr io.Writer) deferRoute {
	if !in.Deferred || len(committed) == 0 {
		return deferRoute{}
	}
	d, ok := core.RecordedDeferDecision(in.Workspace)
	if !ok {
		fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: cycle %d has no recorded defer-or-quarantine failure-decision.json; the items are released\n", in.Cycle)
		return deferRoute{}
	}
	reason := fmt.Sprintf("cycle %d ended by a defer-or-quarantine decision: category %s, fix_type %s", in.Cycle, d.Category, d.FixType)
	switch d.FixType {
	case policy.FixTypeReintent:
		return deferRoute{routed: routeEachConsole(opts, committed, reason, in.Cycle)}
	case policy.FixTypePipelineRepair:
		id, err := fileCompanion(opts, in.Cycle, reason, committed)
		if err != nil {
			fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: the companion item %s was not filed and stamped (%v); the items are released\n", id, err)
			return deferRoute{}
		}
		return deferRoute{routed: true, companion: id}
	default:
		fmt.Fprintf(stderr, "[cycleoutcome] WARN: defer route: fix_type %q has no route; the items are released\n", d.FixType)
		return deferRoute{}
	}
}

func routeEachConsole(opts inboxmover.Options, ids []string, reason string, cycle int) bool {
	routed := true
	for _, id := range ids {
		if _, err := inboxmover.RouteConsole(opts, id, reason, cycle); err != nil {
			routed = false
		}
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
		"route":          lifecycle.RouteConsoleValue,
		"priority_class": "stability",
		"weight":         companionWeight,
	})
	if _, err := inboxmover.File(opts, raw); err != nil {
		return id, err
	}
	_, err := inboxmover.RouteConsole(opts, id, reason, cycle)
	return id, err
}

func waitOnCompanion(opts inboxmover.Options, ids []string, companion string) error {
	var errs []error
	for _, id := range ids {
		if slices.Contains(inboxmover.ResolveDispatchState(opts, id).Deps, companion) {
			continue
		}
		if _, err := depsEdit(opts, id, []inboxmover.FieldEdit{{Op: inboxmover.EditAdd, Field: "deps", Value: companion}}); err != nil {
			errs = append(errs, fmt.Errorf("add deps %s to %s: %w", companion, id, err))
		}
	}
	return errors.Join(errs...)
}
