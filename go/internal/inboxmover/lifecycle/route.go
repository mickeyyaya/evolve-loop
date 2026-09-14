package lifecycle

// route.go — RouteConsole: the FAIL closeout's per-item breaker for a
// deterministic refusal. A triage gate that refuses a top_n card because it
// names a protected surface has said, in so many words, that the item is
// operator-owned; retrying it lane after lane only burns the scout and triage
// tokens again (nine cycles for one item, 1650–1675 — the incident doc). The
// item is rewritten IN PLACE, wherever the lane's claim left it, so the drain
// that follows releases it already carrying route:console-manual and the
// ADR-0074 claim floor refuses every later lane.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

// RouteConsoleValue is the route the breaker writes — the same word the
// operator writes by hand and the claim floor reads (inboxbatch.ConsoleRouted).
const RouteConsoleValue = "console-manual"

// RouteResult describes what happened: the rewritten item's path.
type RouteResult struct {
	Path string
}

// RouteConsole rewrites the item's record with route:console-manual, the
// refusal as routed_reason, the cycle that refused it and the clock's stamp;
// the item is located across processing/cycle-*/ and the inbox root (Locate)
// and is NOT moved. Returns ErrNotFound (INBOX_ROUTE_NOT_FOUND) when no record
// carries the id and the rewrite fault (INBOX_ITEM_REWRITE_FAILED, step=route)
// when the atomic rewrite cannot happen — in both cases nothing is routed and
// the fault is on the stream.
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
	// An item held by ANOTHER cycle's claim is a live lane's, not this closeout's
	// to route (a mis-copied id must never rewrite someone else's work).
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

// jsonString renders a Go string as a JSON string value for the item rewrite.
func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s) // a string never fails to marshal
	return b
}
