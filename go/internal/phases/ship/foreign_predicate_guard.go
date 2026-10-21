package ship

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

var predicatePackageCycle = regexp.MustCompile(`^go/acs/cycle([0-9]+)/`)

func refuseForeignPredicatePackages(ctx context.Context, opts *Options, revs ...string) error {
	if opts.Class != ClassCycle {
		return nil
	}
	cycle, ok, err := cycleIDForShip(opts)
	if err != nil {
		return shipErr(core.CodeStateIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: read the shipping cycle for the predicate package check: "+err.Error())
	}
	if !ok {
		return nil
	}
	args := []string{"diff", "--name-only", "-z", "--no-renames", "--diff-filter=A"}
	args = append(args, revs...)
	args = append(args, "--", "go/acs/")
	out, err := captureGitOutput(ctx, opts, args...)
	if err != nil {
		return err
	}
	foreign := foreignPredicateFiles(out, cycle)
	if len(foreign) == 0 {
		return nil
	}
	return shipErr(shiperr.CodeForeignPredicatePackage, core.ShipClassPrecondition, core.StageAtomicShip,
		"ship: cycle "+strconv.Itoa(cycle)+" refuses to land the predicate package of another cycle: "+strings.Join(foreign, ", ")+
			"; archive it with its explanation record under docs/private/research/archived-YYYY-MM-DD/superseded-predicate-packages/",
		"paths", strings.Join(foreign, ","), "cycle", strconv.Itoa(cycle))
}

func foreignPredicateFiles(nameOnly string, cycle int) []string {
	own := strconv.Itoa(cycle)
	var foreign []string
	for _, path := range strings.Split(nameOnly, "\x00") {
		m := predicatePackageCycle.FindStringSubmatch(path)
		if m != nil && m[1] != own {
			foreign = append(foreign, path)
		}
	}
	return foreign
}
