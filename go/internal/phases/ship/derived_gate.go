package ship

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func refuseStaleDerived(ctx context.Context, opts *Options, res *RunResult) error {
	root, baseRef, err := repoContractGateRoot(opts)
	if err != nil {
		return err
	}
	files, err := changedFilesTwice(opts.Stderr, root, baseRef)
	if err != nil {
		return err
	}
	lines, err := core.StaleDerivedFindings(ctx, root, changedPaths(files), opts.derivedGenerator(root), opts.Stderr)
	switch {
	case err != nil:
		return shipErr(shiperr.CodeDerivedStale, core.ShipClassTransient, core.StageVerifyClass,
			"ship: the derived-file checks could not run, so the tree is not judged: "+err.Error())
	case len(lines) > 0:
		return shipErr(shiperr.CodeDerivedStale, core.ShipClassPrecondition, core.StageVerifyClass, strings.Join(lines, "\n"))
	}
	res.Logs = append(res.Logs, "[ship] OK: every derived file the cycle diff touches passes its check verb")
	return nil
}

var derivedGeneratorFor core.DerivedRunFn = core.LaneDerivedGenerator

func (o *Options) derivedGenerator(root string) derived.Generator {
	if o.derivedRun != nil {
		return o.derivedRun
	}
	return derivedGeneratorFor(root)
}
