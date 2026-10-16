package inboxmover

import (
	"errors"
	"fmt"
	"strconv"
)

func RetireUnbacked(opts Options, cycle int, state, reason, commitSHA string, ids []string) ([]string, error) {
	opts.resolveOpts()
	mover := opts.mover()
	promote := PromoteOpts{Cycle: strconv.Itoa(cycle), CommitSHA: commitSHA}
	var retired []string
	var errs []error
	for _, id := range dedupeIDs(ids) {
		if ResolveDispatchState(opts, id).State != StateUnknown {
			continue
		}
		if _, err := mover.RetireUnbacked(id, state, promote, reason); err != nil {
			errs = append(errs, fmt.Errorf("retire-unbacked %q: %w", id, err))
			continue
		}
		retired = append(retired, id)
	}
	return retired, errors.Join(errs...)
}
