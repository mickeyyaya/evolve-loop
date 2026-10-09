package filter

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Catalog struct {
	Kinds   []signalcenter.Kind
	Modules []signalcenter.Module
	Codes   []signalcenter.Code
}

func RegisteredCatalog() Catalog {
	codes := []signalcenter.Code{}
	for _, docs := range signalcenter.RegisteredCodes() {
		for _, d := range docs {
			codes = append(codes, d.Code)
		}
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return Catalog{Kinds: signalcenter.Kinds(), Modules: signalcenter.Modules(), Codes: codes}
}

func (c Catalog) kindNames() []string { return names(c.Kinds) }

func (c Catalog) moduleNames() []string { return names(c.Modules) }

func (c Catalog) codeNames() []string { return names(c.Codes) }

func names[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}
