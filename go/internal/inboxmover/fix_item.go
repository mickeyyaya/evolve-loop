package inboxmover

import (
	"errors"
	"fmt"
)

func ConsumeFixItem(opts Options, id string, p PromoteOpts) error {
	opts.resolveOpts()
	_, err := Locate(opts.InboxDir, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err == nil {
		_, err = ReconcileSuperseded(opts, []string{id}, StateProcessed, p)
	}
	if err != nil {
		return fmt.Errorf("fix item %s: %w", id, err)
	}
	return nil
}
