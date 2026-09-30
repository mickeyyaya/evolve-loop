package inboxmover

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
)

var ErrWaitingOnDependency = errors.New("inboxmover: item waits on a dependency that has not landed — refusing claim")

// ClaimPending claims the root-pending ids, leaving absent, held and console-routed ones for the effects gate.
func ClaimPending(opts Options, cycle int, ids []string) error {
	opts.resolveOpts()
	cycleStr := strconv.Itoa(cycle)
	var errs []error
	for _, id := range dedupeIDs(ids) {
		loc, err := Locate(opts.InboxDir, id)
		switch {
		case errors.Is(err, ErrNotFound):
			continue
		case err != nil:
			errs = append(errs, fmt.Errorf("locate %q: %w", id, err))
			continue
		case loc.Cycle != 0:
			continue
		}
		if _, err := ClaimDispatchable(opts, id, cycleStr); err != nil && !errors.Is(err, ErrConsoleRouted) && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrWaitingOnDependency) {
			errs = append(errs, fmt.Errorf("claim %q: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func ClaimDispatchable(opts Options, taskID, cycle string) (ClaimResult, error) {
	opts.resolveOpts()
	if ds := ResolveDispatchState(opts, taskID); ds.State == StatePending {
		if d := PendingDispatchability(opts, ds.Deps); !d.Dispatchable {
			fmt.Fprintf(opts.Stderr, lifecycle.LegacyPrefix+"refusing claim of %q for cycle %s: %s — it stays pending until that dependency lands\n", taskID, cycle, d.Reason)
			return ClaimResult{}, fmt.Errorf("%w: %s: %s", ErrWaitingOnDependency, taskID, d.Reason)
		}
	}
	return Claim(opts, taskID, cycle)
}
