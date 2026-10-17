package codeindextest

import "github.com/mickeyyaya/evolve-loop/go/internal/codeindex"

const (
	probeCallerFile = "go/cmd/evolve/cmd_models_live.go"
	probeMapDir     = "landing/"
)

func StandardProbes() []Probe {
	return []Probe{
		{
			Request:    codeindex.Request{Query: codeindex.QueryCallers, Arg: "runStagedCatalogRefresh", Direction: codeindex.DirectionIn},
			WantSymbol: "makeCatalogRefresher",
			Missing:    codeindex.Request{Query: codeindex.QueryCallers, Arg: "zzzNoSuchSym", Direction: codeindex.DirectionIn},
		},
		{
			Request:    codeindex.Request{Query: codeindex.QueryAPI, Arg: probeCallerFile},
			WantSymbol: "shouldRefreshCatalog",
			Missing:    codeindex.Request{Query: codeindex.QueryAPI, Arg: "nosuch.go"},
		},
		{
			Request:    codeindex.Request{Query: codeindex.QueryFind, Arg: "catalog refresh"},
			WantSymbol: "runStagedCatalogRefresh",
			Missing:    codeindex.Request{Query: codeindex.QueryFind, Arg: "qqqzzzxxx"},
		},
		{
			Request:  codeindex.Request{Query: codeindex.QueryMap, Arg: probeMapDir},
			WantFile: "landing/cmd/",
			Missing:  codeindex.Request{Query: codeindex.QueryMap, Arg: "nosuchdir/"},
		},
	}
}

func StandardIndex() map[codeindex.Request][]codeindex.Hit {
	index := map[codeindex.Request][]codeindex.Hit{}
	for _, probe := range StandardProbes() {
		index[probe.Request] = []codeindex.Hit{{
			File:   firstNonEmpty(probe.WantFile, probeCallerFile),
			Line:   1,
			Symbol: probe.WantSymbol,
			Text:   "standard fixture",
		}}
	}
	return index
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
