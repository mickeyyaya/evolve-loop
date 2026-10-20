package ship

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func enterShipWindow(ctx context.Context, opts *Options, res *RunResult) func() {
	if opts.Class != ClassCycle || opts.DryRun {
		return func() {}
	}
	lease, err := shipwindow.Acquire(ctx, filepath.Join(opts.ProjectRoot, ".evolve"), shipwindow.Options{})
	if err != nil {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: ship-window lease not acquired (%v); proceeding unleased", err))
		return func() {}
	}
	return func() {
		if err := lease.Release(); err != nil {
			res.Logs = append(res.Logs, "[ship] WARN: ship-window lease release: "+err.Error())
		}
	}
}
