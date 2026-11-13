package mainstop

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
)

func ProofReds(keys []baseattr.Key) baseattr.Reds {
	return baseattr.Reds{Keys: keys, Flags: covchanged.TagFlags(covchanged.DefaultTags)}
}
