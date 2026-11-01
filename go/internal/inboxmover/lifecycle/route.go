package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const RouteConsoleValue = "console-manual"

type RouteResult struct {
	Path string
}

type RouteRequest struct {
	TaskID    string
	Reason    string
	Cycle     int
	BlockedBy string
}

func (m *Mover) RouteConsole(taskID, reason string, cycle int) (RouteResult, error) {
	return m.RouteConsoleRequest(RouteRequest{TaskID: taskID, Reason: reason, Cycle: cycle})
}

func (m *Mover) RouteConsoleRequest(r RouteRequest) (RouteResult, error) {
	taskID, reason, cycle := r.TaskID, r.Reason, r.Cycle
	res := RouteResult{}
	loc, err := m.locateConsoleRoutable(taskID, cycle)
	if err != nil {
		return res, err
	}
	if err := m.checkBlocker(taskID, r.BlockedBy); err != nil {
		return res, err
	}
	stamp := m.now().UTC().Format(time.RFC3339)
	if rerr := rewriteItemJSON(loc.Path, admitEvery, func(item map[string]json.RawMessage) error {
		item[RouteField] = jsonString(RouteConsoleValue)
		item[inboxbatch.RoutedReasonField] = jsonString(reason)
		item[inboxbatch.RoutedCycleField] = json.RawMessage(strconv.Itoa(cycle))
		item[inboxbatch.RoutedAtField] = jsonString(stamp)
		return addBlocker(item, r.BlockedBy)
	}); errors.Is(rerr, ErrInvalidItem) {
		return res, fmt.Errorf("route-console: %s: %w", taskID, rerr)
	} else if rerr != nil {
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

func (m *Mover) locateConsoleRoutable(taskID string, cycle int) (Location, error) {
	if taskID == "" {
		m.linef("ERROR: usage: route-console <task_id> <reason> <cycle>")
		return Location{}, fmt.Errorf("%w: route-console requires task_id", ErrBadArgs)
	}
	loc, err := Locate(m.inboxDir, taskID)
	if err != nil {
		m.warn(fault{code: CodeRouteNotFound, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("route-console: task '%s' not found in %s — the refusal cannot be recorded on the item", taskID, m.inboxDir),
			fields: map[string]string{"task_id": taskID, "inbox_dir": m.inboxDir, "step": "locate"}})
		return Location{}, fmt.Errorf("%w: %s", ErrNotFound, taskID)
	}
	if loc.Cycle != 0 && loc.Cycle != cycle {
		m.warn(fault{code: CodeRouteNotFound, origin: "Mover.RouteConsole", cycle: cycle, legacy: "WARN: ",
			reason: fmt.Sprintf("route-console: task '%s' is held by cycle %d's claim, not cycle %d's — not routed", taskID, loc.Cycle, cycle),
			fields: map[string]string{"task_id": taskID, "inbox_dir": m.inboxDir, "held_by_cycle": strconv.Itoa(loc.Cycle), "step": "locate"}})
		return Location{}, fmt.Errorf("%w: %s (held by cycle %d)", ErrNotFound, taskID, loc.Cycle)
	}
	return loc, nil
}

func (m *Mover) RouteLane(taskID, reason string) (RouteResult, error) {
	return m.RouteLaneRequest(RouteRequest{TaskID: taskID, Reason: reason})
}

func (m *Mover) RouteLaneRequest(r RouteRequest) (RouteResult, error) {
	taskID, reason := r.TaskID, r.Reason
	res := RouteResult{}
	loc, err := m.locatePending("route-lane", taskID)
	if err != nil {
		return res, err
	}
	if err := m.checkBlocker(taskID, r.BlockedBy); err != nil {
		return res, err
	}
	stamp := m.now().UTC().Format(time.RFC3339)
	admit := func(body []byte) error { return m.admitToLanes(taskID, loc.Path, body) }
	if err := rewriteItemJSON(loc.Path, admit, func(it map[string]json.RawMessage) error {
		it[RouteField] = jsonString(inboxbatch.RouteLaneValue)
		it[inboxbatch.RoutedReasonField] = jsonString(withEarlierReason(reason, it[inboxbatch.RoutedReasonField]))
		it[inboxbatch.RoutedAtField] = jsonString(stamp)
		delete(it, inboxbatch.RoutedCycleField)
		return addBlocker(it, r.BlockedBy)
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

func (m *Mover) checkBlocker(taskID, blocker string) error {
	if blocker == "" {
		return nil
	}
	filed, err := filedItemIDs(m.inboxDir)
	if err != nil {
		return fmt.Errorf("blocked-by: scan %s: %w", m.inboxDir, err)
	}
	return checkCuratedIdentity(inboxbatch.Item{ID: taskID, Deps: []string{blocker}}, map[string]bool{"deps": true}, filed)
}

func addBlocker(item map[string]json.RawMessage, blocker string) error {
	if blocker == "" || listsDep(item, blocker) {
		return nil
	}
	return editList(item, FieldEdit{Op: EditAdd, Field: "deps", Value: blocker})
}

func listsDep(item map[string]json.RawMessage, dep string) bool {
	var deps []string
	return json.Unmarshal(item["deps"], &deps) == nil && slices.Contains(deps, dep)
}

func withEarlierReason(reason string, earlier json.RawMessage) string {
	var prior string
	if json.Unmarshal(earlier, &prior) != nil || strings.TrimSpace(prior) == "" {
		return reason
	}
	return reason + "; earlier reason: " + prior
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
