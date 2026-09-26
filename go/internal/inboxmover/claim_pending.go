package inboxmover

import (
	"errors"
	"fmt"
	"strconv"
)

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
		if _, err := Claim(opts, id, cycleStr); err != nil && !errors.Is(err, ErrConsoleRouted) && !errors.Is(err, ErrNotFound) {
			errs = append(errs, fmt.Errorf("claim %q: %w", id, err))
		}
	}
	return errors.Join(errs...)
}
