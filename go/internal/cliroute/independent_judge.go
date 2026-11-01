package cliroute

import (
	"errors"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

var ErrNoIndependentFamily = errors.New("no independent family")

func (d Decision) IndependentOf(family string) (Decision, error) {
	candidates := slices.DeleteFunc(slices.Clone(d.Plan.Candidates), func(cli string) bool { return llmroute.Family(cli) == family })
	if len(candidates) == 0 {
		return d, fmt.Errorf("%w: each candidate of %v is of the builder family %s", ErrNoIndependentFamily, d.Plan.Candidates, family)
	}
	d.Plan.Candidates = candidates
	return d, nil
}
