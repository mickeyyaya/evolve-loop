package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

var qualityCompareHits = map[string]func(pipelinerates.Cycle) bool{
	"PQ1": pipelinerates.Shipped,
	"PQ4": pipelinerates.Cycle.DeadlineEnd,
}

type qualityCompareArgs struct {
	index, root string
	a, b        pipelinerates.Window
	paired      bool
}

func runQualityCompare(env qualityEnv, args []string, stdout, stderr io.Writer) int {
	opts, err := parseQualityCompare(args)
	if err != nil {
		fmt.Fprintf(stderr, "quality compare: %v\n%s\n", err, qualityUsage)
		return exitUsage
	}
	cycles, err := pipelinerates.LoadCycles(opts.root)
	if err != nil {
		fmt.Fprintf(stderr, "quality compare: %v\n", err)
		return exitIO
	}
	hit := qualityCompareHits[opts.index]
	ka, na := pipelinerates.RateIn(cycles, opts.a, hit)
	kb, nb := pipelinerates.RateIn(cycles, opts.b, hit)
	c := pipelinerates.Compare(pipelinerates.Rate{K: ka, N: na}, pipelinerates.Rate{K: kb, N: nb})
	fmt.Fprintf(stdout, "quality compare: %s a=%s %d/%d b=%s %d/%d diff=%+.3f mde=%.3f status=%s\n", opts.index, opts.a.Name, ka, na, opts.b.Name, kb, nb, c.Diff, c.MDE, c.Status)
	if opts.paired {
		onlyA, onlyB := pipelinerates.Discordant(cycles, opts.a, opts.b)
		fmt.Fprintf(stdout, "quality compare: paired items shipped only in a=%d only in b=%d mcnemar_p=%.4f\n", onlyA, onlyB, pipelinerates.McNemarExact(onlyA, onlyB))
	}
	center, closeCenter := env.newCenter(opts.root, paths.EvolveDirOf(opts.root), stderr)
	defer closeCenter()
	t := qualityTally{center: center}
	t.publish([]qualitypanel.Reading{c.Reading(opts.index + " a:" + opts.a.Name + "|b:" + opts.b.Name)}, t.stages(opts.root))
	return 0
}

func parseQualityCompare(args []string) (qualityCompareArgs, error) {
	var o qualityCompareArgs
	var a, b string
	operands, err := (cliFlags{values: map[string]*string{"--index": &o.index, "--a": &a, "--b": &b, "--project-root": &o.root}, bools: map[string]*bool{"--paired": &o.paired}}).parse(args)
	switch {
	case err != nil:
		return o, err
	case len(operands) > 0:
		return o, fmt.Errorf("unexpected argument %q", operands[0])
	case o.root == "":
		return o, errors.New("--project-root is required")
	case qualityCompareHits[o.index] == nil:
		return o, fmt.Errorf("index %q is not PQ1 or PQ4", o.index)
	}
	if o.a, err = pipelinerates.ParseWindow(a); err == nil {
		o.b, err = pipelinerates.ParseWindow(b)
	}
	return o, err
}
