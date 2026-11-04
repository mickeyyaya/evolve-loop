package commitgate

import (
	"context"
	"errors"
	"io/fs"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

func (o Options) refuseDroppedJourneys(ctx context.Context, files []string, res *Result) int {
	if !slices.Contains(files, journeys.RegistryPath) {
		return ExitPass
	}
	head, err := journeys.Load(o.RepoRoot)
	if err != nil {
		res.log("REJECTED: the journey registry %s: %v", journeys.RegistryPath, err)
		return ExitFail
	}
	raw, err := commentaudit.ReadAtBase(o.git(ctx), "HEAD")(journeys.RegistryPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return ExitPass
	case err != nil:
		res.log("cannot read the journey registry at HEAD: %v", err)
		return ExitGitFatal
	}
	base, err := journeys.Parse(raw)
	if err != nil {
		res.log("the journey registry at HEAD does not parse, so no row of HEAD is checked: %v", err)
		return ExitPass
	}
	if dropped := head.Removed(base); len(dropped) > 0 {
		res.log("REJECTED: the journey rows %v left %s; a row goes away only with retired and a reason", dropped, journeys.RegistryPath)
		return ExitFail
	}
	return ExitPass
}
