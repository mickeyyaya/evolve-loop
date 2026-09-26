package cycleoutcome

// nowork.go — the planned-no-work closeout (F30), the sibling of ApplyFailure.
//
// A fleet lane whose triage answered for its scoped item without committing it
// (committedset.Dispositions: a reasoned drop, a skip, an escalation) ends as
// planned no-work. The item is still pending in the inbox root — triage builds
// its menu there — so the next wave would draw it into another lane and
// another no-work end. This closeout hands it to the console IN PLACE with the
// lane's reason, and the ADR-0074 claim floor refuses every later lane. The
// console confirms and retires it: an unshipped lane never retires work (the
// anti-laundering principle behind the closure-claim gate).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// NoWorkResult is what the no-work closeout did to the lane's scoped ids.
type NoWorkResult struct {
	Routed  []string // scoped inbox items handed to the console
	Retired []string // scoped ids no inbox item backs, retired as rejected/cycle-N/ records
}

// NoWorkInputs is one planned-no-work cycle's closeout context.
type NoWorkInputs struct {
	ProjectRoot string    // repo root containing .evolve/
	Workspace   string    // .evolve/runs/cycle-N
	Cycle       int       // cycle number
	Stderr      io.Writer // nil = discard
	// Ledger and Signals are the root's observed seams, as on FailureInputs.
	Ledger  inboxmover.LedgerAppender
	Signals *signalcenter.Center
}

// WithLedger returns the inputs with the lifecycle ledger set (the receiver
// is left untouched).
func (in NoWorkInputs) WithLedger(l inboxmover.LedgerAppender) NoWorkInputs {
	in.Ledger = l
	return in
}

// WithSignals returns the inputs with the Signal Center set (the receiver is
// left untouched).
func (in NoWorkInputs) WithSignals(c *signalcenter.Center) NoWorkInputs {
	in.Signals = c
	return in
}

// ApplyNoWork hands every lane-scoped item the lane's triage answered for to
// the console and returns the ids it routed; then — the sibling of
// ApplyFailure's routed drain — it releases the cycle's claims to the inbox
// root with no failure bump, because triage claims before it selects and a
// no-work lane worked nothing (F30 architecture review C1): a routed item
// arrives console-visible instead of stranded in the gitignored claim dir.
// A cycle without a lane pin has no scope to hand over and releases nothing
// (a sequential no-work end keeps its historical lifecycle). An answered id
// OUTSIDE the scope is never touched (an agent-authored id never widens what
// a closeout may do — ApplyFailure's rule); one already routed to the console
// keeps its original evidence (review m3); one no longer in the inbox (a ship
// consumed it, or it was never an inbox item) is simply gone, not a route
// failure (review m4). Route and release faults are on the stream and are
// returned joined, for the caller to WARN — a lifecycle hiccup never changes
// a cycle's exit code.
func ApplyNoWork(in NoWorkInputs) (NoWorkResult, error) {
	res := NoWorkResult{}
	scope := LaneScopeIDs(in.Workspace)
	if len(scope) == 0 {
		return res, nil
	}
	stderr := in.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	opts := inboxmover.Options{ProjectRoot: in.ProjectRoot, Ledger: in.Ledger, Stderr: stderr, Signals: in.Signals}
	inboxDir := filepath.Join(in.ProjectRoot, ".evolve", "inbox")
	var errs []error
	answers := map[string]string{}
	for _, d := range committedset.Dispositions(in.Workspace) {
		if !containsID(scope, d.ID) {
			continue
		}
		answers[d.ID] = noWorkRouteReason(in.Cycle, d)
		if present, alreadyRouted := itemRouteState(inboxDir, d.ID); !present || alreadyRouted {
			continue
		}
		if _, err := inboxmover.RouteConsole(opts, d.ID, answers[d.ID], in.Cycle); err != nil {
			if !errors.Is(err, inboxmover.ErrNotFound) {
				errs = append(errs, err)
			}
			continue
		}
		res.Routed = append(res.Routed, d.ID)
	}
	retired, err := retireUnbackedScope(opts, in.Cycle, scope, answers)
	res.Retired = retired
	if err != nil {
		errs = append(errs, err)
	}
	if _, err := inboxmover.ReleaseCycleProcessingWithReason(opts, in.Cycle, "planned no-work: the lane's triage answered without committing"); err != nil {
		errs = append(errs, err)
	}
	return res, errors.Join(errs...)
}

// retireUnbackedScope retires every scoped id no inbox item backs as rejected, each with the lane's own
// answer for it or the statement that the lane answered for nothing; the next wave then plans around it.
func retireUnbackedScope(opts inboxmover.Options, cycle int, scope []string, answers map[string]string) ([]string, error) {
	var retired []string
	var errs []error
	for _, id := range scope {
		reason, answered := answers[id]
		if !answered {
			reason = fmt.Sprintf("planned no-work (cycle %d): the lane's triage answered for nothing", cycle)
		}
		ids, err := inboxmover.RetireUnbacked(opts, cycle, inboxmover.StateRejected, reason, "", []string{id})
		retired = append(retired, ids...)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return retired, errors.Join(errs...)
}

// itemRouteState locates an inbox item (root or any claim dir) and reports
// whether it exists and already carries a console route stamp.
func itemRouteState(inboxDir, id string) (present, consoleRouted bool) {
	loc, err := inboxmover.Locate(inboxDir, id)
	if err != nil {
		return false, false
	}
	body, err := os.ReadFile(loc.Path)
	if err != nil {
		return true, false
	}
	var item struct {
		Route string `json:"route"`
	}
	_ = json.Unmarshal(body, &item) // a record that cannot be parsed is not console-stamped
	return true, strings.HasPrefix(item.Route, "console")
}

// noWorkRouteReason is the routed_reason the console reads: the cycle, the
// bucket and the lane's own evidence.
func noWorkRouteReason(cycle int, d committedset.Disposition) string {
	reason := fmt.Sprintf("lane triage (cycle %d) %s", cycle, d.Bucket)
	if d.Reason != "" {
		reason += ": " + d.Reason
	}
	return reason
}
