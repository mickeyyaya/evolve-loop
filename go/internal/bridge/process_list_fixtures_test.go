package bridge

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func failingList(err error) func(context.Context) ([]proctree.Process, error) {
	return func(context.Context) ([]proctree.Process, error) { return nil, err }
}
