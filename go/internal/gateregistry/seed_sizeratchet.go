package gateregistry

import (
	"context"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
)

func sizeratchetEntry() gatecontract.Entry {
	return gatecontract.Entry{
		Name:   "size-ratchet",
		Kind:   gatecontract.KindScanner,
		Files:  []string{"go/internal/sizeratchet/"},
		Owners: []gatecontract.Owner{{Pattern: "*", Phase: "build"}},
		Inputs: []gatecontract.Declared{
			{Path: "go/", Class: gatecontract.ClassSubject},
			{Path: "go/" + sizeratchet.OffendersRelPath, Class: gatecontract.ClassAuthority},
		},
		Stage: gatecontract.StageEnforce,
		Run:   runSizeratchet,
	}
}

func runSizeratchet(_ context.Context, in gatecontract.Input) (gatecontract.Verdict, error) {
	spans, offenders, err := sizeratchet.Load(filepath.Join(in.Root, "go"))
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	return verdictOf(sizeratchet.Check(spans, offenders)), nil
}
