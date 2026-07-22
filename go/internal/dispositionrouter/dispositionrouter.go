// Package dispositionrouter is S3 of the failure-disposition-router design:
// the deterministic floor layer that decides where a classified failure goes
// (operator console vs the priority queue) and the staging writer that records
// the resulting intents for the boundary applier to consume.
//
// Two invariants own this package:
//
//	(1) FLOORS ARE DETERMINISTIC AND ONE-WAY. An advisory LLM route may RAISE a
//	    queue disposition to console, but may never LOWER a floor-forced console
//	    back to the queue. A severed statemap (guard-abort) and a defect that
//	    survived two fixes (recurrence >= 3) are operator-owned by construction.
//
//	(2) STAGING NEVER WRITES THE INBOX. Intents land in
//	    .evolve/escalations/pending-actions.jsonl, never in .evolve/inbox/.
//	    A mid-flight inbox write races inboxmover.Claim's os.Rename and
//	    resurrects a claimed item into double work across fleet lanes; only the
//	    boundary applier (recurrence.ApplyBoundary), running after a wave has
//	    drained, may touch the inbox.
//
// Named `dispositionrouter` rather than `router`: internal/router is the
// unrelated phase/model dispatch router, and conflating two routers in one
// package would be a namespace collision, not reuse.
//
// Leaf package: stdlib + internal/adapters/flock only.
package dispositionrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// Route vocabulary. Closed set: a disposition is either operator-owned
// (console) or lane-dispatchable (queue).
const (
	RouteConsole = "console"
	RouteQueue   = "queue"
)

// GuardAbortClass is the pre-classification whose floor is unconditional: a
// guard abort is pipeline machinery failing by construction (a severed
// statemap), never a lane-sized task defect.
const GuardAbortClass = "guard-abort"

// RecurrenceConsoleFloor is the recurrence count at or above which a failure is
// force-routed to the console: a defect that survived two fixes is no longer a
// lane-sized task, and re-queueing it a third time only reproduces the fault.
const RecurrenceConsoleFloor = 3

// PendingActionsFile is the staging file's basename under the escalations dir.
const PendingActionsFile = "pending-actions.jsonl"

// Decision is one routing outcome. Forced marks a floor decision — the
// advisory layer may not lower it. Reason always explains a forced decision.
type Decision struct {
	Route  string
	Forced bool
	Reason string
}

// Decide resolves the disposition for a failure of pre-class preClass seen
// recurrence times, given the advisory llmRoute ("" for no advice).
//
// Floors first (forced console), then the advisory route, which may only raise
// queue -> console. An unknown or empty llmRoute is a no-op.
func Decide(preClass string, recurrence int, llmRoute string) Decision {
	if preClass == GuardAbortClass {
		return Decision{
			Route:  RouteConsole,
			Forced: true,
			Reason: "floor: guard-abort pre-class is operator-owned (pipeline machinery, not a lane task)",
		}
	}
	if recurrence >= RecurrenceConsoleFloor {
		return Decision{
			Route:  RouteConsole,
			Forced: true,
			Reason: fmt.Sprintf("floor: recurrence %d >= %d — the defect survived prior fixes", recurrence, RecurrenceConsoleFloor),
		}
	}
	if llmRoute == RouteConsole {
		return Decision{Route: RouteConsole, Reason: "advisory raise: queue -> console"}
	}
	return Decision{Route: RouteQueue}
}

// Intent is one staged action for the boundary applier: either escalate the
// weight of an existing open inbox item, or autofile a new one for a pattern
// with no open item. Weight is the base weight the escalation formula starts
// from; Recurrence is the count it escalates for.
type Intent struct {
	Cycle      int     `json:"cycle"`
	Pattern    string  `json:"pattern"`
	ItemID     string  `json:"item_id,omitempty"`
	Action     string  `json:"action"` // "escalate" | "autofile"
	Route      string  `json:"route"`
	Recurrence int     `json:"recurrence"`
	Weight     float64 `json:"weight"`
	Reason     string  `json:"reason,omitempty"`
}

// Action vocabulary for Intent.Action.
const (
	ActionEscalate = "escalate"
	ActionAutofile = "autofile"
)

// PendingActionsPath returns the staging file path under escalationsDir.
func PendingActionsPath(escalationsDir string) string {
	return filepath.Join(escalationsDir, PendingActionsFile)
}

// StageIntent appends in as one JSONL record to the pending-actions file under
// escalationsDir (created on first run) and returns that path. It never writes
// .evolve/inbox/ — see invariant (2) in the package doc. The append is
// serialized by the shared file lock so concurrent lanes cannot tear a line.
func StageIntent(escalationsDir string, in Intent) (string, error) {
	path := PendingActionsPath(escalationsDir)
	if err := os.MkdirAll(escalationsDir, 0o755); err != nil {
		return "", fmt.Errorf("dispositionrouter: create escalations dir: %w", err)
	}
	line, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("dispositionrouter: encode intent: %w", err)
	}
	err = flock.WithPathLock(path, func() error {
		f, oerr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if oerr != nil {
			return fmt.Errorf("dispositionrouter: open staging file: %w", oerr)
		}
		defer f.Close()
		if _, werr := f.Write(append(line, '\n')); werr != nil {
			return fmt.Errorf("dispositionrouter: append intent: %w", werr)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// LoadIntents reads every staged intent from the pending-actions file at path.
// A missing file yields (nil, nil) — nothing staged is the common case, not an
// error. A malformed line is a real error the caller must surface.
func LoadIntents(path string) ([]Intent, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dispositionrouter: read %s: %w", path, err)
	}
	var out []Intent
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var in Intent
		if derr := dec.Decode(&in); derr != nil {
			return nil, fmt.Errorf("dispositionrouter: parse %s: %w", path, derr)
		}
		out = append(out, in)
	}
	return out, nil
}
