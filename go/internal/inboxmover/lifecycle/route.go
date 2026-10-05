package lifecycle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

// RouteConsoleValue is the route RouteConsole writes and the claim floor refuses.
const RouteConsoleValue = "console-manual"

// RouteResult describes what happened: the rewritten item's path.
type RouteResult struct {
	Path string
}

// RouteConsole marks taskID's item route:console-manual where it lies, so no later lane claims it.
func (m *Mover) RouteConsole(taskID, reason string, cycle int) (RouteResult, error) {
	res := RouteResult{}
	if taskID == "" {
		m.linef("ERROR: usage: route-console <task_id> <reason> <cycle>")
		return res, fmt.Errorf("%w: route-console requires task_id", ErrBadArgs)
	}
	loc, err := Locate(m.inboxDir, taskID)
	if err != nil {
		m.warn(fault{code: CodeRouteNotFound, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("route-console: task '%s' not found in %s — the refusal cannot be recorded on the item", taskID, m.inboxDir),
			fields: map[string]string{"task_id": taskID, "inbox_dir": m.inboxDir, "step": "locate"}})
		return res, fmt.Errorf("%w: %s", ErrNotFound, taskID)
	}
	// Another cycle's claim belongs to a live lane; a mis-copied id must never rewrite it.
	if loc.Cycle != 0 && loc.Cycle != cycle {
		m.warn(fault{code: CodeRouteNotFound, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("route-console: task '%s' is held by cycle %d's claim, not cycle %d's — not routed", taskID, loc.Cycle, cycle),
			fields: map[string]string{"task_id": taskID, "inbox_dir": m.inboxDir, "held_by_cycle": strconv.Itoa(loc.Cycle), "step": "locate"}})
		return res, fmt.Errorf("%w: %s (held by cycle %d)", ErrNotFound, taskID, loc.Cycle)
	}
	stamp := m.now().UTC().Format(time.RFC3339)
	if rerr := UpdateItemJSON(loc.Path, func(item map[string]json.RawMessage) {
		item[RouteField] = jsonString(RouteConsoleValue)
		item[inboxbatch.RoutedReasonField] = jsonString(reason)
		item[inboxbatch.RoutedCycleField] = json.RawMessage(strconv.Itoa(cycle))
		item[inboxbatch.RoutedAtField] = jsonString(stamp)
	}); rerr != nil {
		m.warn(fault{code: CodeItemRewriteFailed, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("route-console: rewrite failed for '%s' (%v) — not routed, it WILL be re-picked", taskID, rerr),
			fields: map[string]string{"task_id": taskID, "path": loc.Path, "err": rerr.Error(), "step": "route"}})
		return res, fmt.Errorf("route-console: rewrite %s: %w", loc.Path, rerr)
	}
	res.Path = loc.Path
	base := filepath.Base(loc.Path)
	m.linef("routed %s: %s (cycle %d)", RouteConsoleValue, base, cycle)
	m.ledgerLine(ledgerEntry{
		Action: "route-console",
		TaskID: taskID,
		Cycle:  intPtr(strconv.Itoa(cycle)),
		Reason: reason,
	})
	m.warn(fault{code: CodeItemRoutedConsole, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
		reason: fmt.Sprintf("route-console: '%s' is now %s — %s", taskID, RouteConsoleValue, reason),
		fields: map[string]string{"task_id": taskID, "path": loc.Path, "reason": reason, "step": "route"}})
	return res, nil
}

func (m *Mover) RouteLane(taskID, reason string) (RouteResult, error) {
	res := RouteResult{}
	loc, err := m.locatePending("route-lane", taskID)
	if err != nil {
		return res, err
	}
	stamp := m.now().UTC().Format(time.RFC3339)
	admit := func(body []byte) error { return m.admitToLanes(taskID, loc.Path, body) }
	if err := updateAdmittedItemJSON(loc.Path, admit, func(it map[string]json.RawMessage) {
		it[RouteField] = jsonString(inboxbatch.RouteLaneValue)
		it[inboxbatch.RoutedReasonField] = jsonString(reason)
		it[inboxbatch.RoutedAtField] = jsonString(stamp)
		delete(it, inboxbatch.RoutedCycleField)
	}); err != nil {
		return res, fmt.Errorf("route-lane: %s: %w", loc.Path, err)
	}
	res.Path = loc.Path
	m.linef("routed %s: %s", inboxbatch.RouteLaneValue, filepath.Base(loc.Path))
	m.ledgerLine(ledgerEntry{Action: "route-lane", TaskID: taskID, Reason: reason})
	return res, nil
}

func (m *Mover) admitToLanes(taskID, path string, body []byte) error {
	item, ok := routingItem(body)
	if !ok {
		return fmt.Errorf("%s is not a well-formed inbox item, so its route cannot be judged", path)
	}
	item.Route = inboxbatch.RouteLaneValue
	if routed, why := inboxbatch.ConsoleRouted(item, m.isProtected); routed {
		return fmt.Errorf("%w: %s stays operator-owned: %s", ErrConsoleRouted, taskID, why)
	}
	return nil
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s) // a string never fails to marshal
	return b
}
