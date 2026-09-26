package lifecycle

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
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
		item["route"] = jsonString(RouteConsoleValue)
		item["routed_reason"] = jsonString(reason)
		item["routed_cycle"] = json.RawMessage(strconv.Itoa(cycle))
		item["routed_at"] = jsonString(stamp)
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

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s) // a string never fails to marshal
	return b
}
