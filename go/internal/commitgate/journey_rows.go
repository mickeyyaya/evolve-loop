package commitgate

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

func (o Options) refuseRemovedJourneyRows(ctx context.Context, files []string, res *Result) int {
	change, err := o.changeAgainstHead(ctx, files)
	if err != nil {
		res.log("journey registry check: %v", err)
		return ExitGitFatal
	}
	if !slices.Contains(change.paths, journeys.RegistryPath) {
		return ExitPass
	}
	before, err := readRegistry(change.before)
	if err != nil {
		res.log("REJECTED: the journey registry %s at HEAD is unreadable: %v", journeys.RegistryPath, err)
		return ExitFail
	}
	after, err := readRegistry(change.after)
	if err != nil {
		res.log("REJECTED: the journey registry %s is unreadable: %v", journeys.RegistryPath, err)
		return ExitFail
	}
	gone := journeys.Removed(before, after)
	if len(gone) == 0 {
		return ExitPass
	}
	res.log("REJECTED: %s loses journey rows %s: a row stays, and a journey that ends keeps its row with a `retired` reason", journeys.RegistryPath, strings.Join(gone, ", "))
	return ExitFail
}

func readRegistry(read func(string) ([]byte, error)) ([]journeys.Row, error) {
	raw, err := read(journeys.RegistryPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return journeys.Parse(raw)
}
