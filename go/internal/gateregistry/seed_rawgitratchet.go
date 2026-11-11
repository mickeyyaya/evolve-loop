package gateregistry

import (
	"context"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet"
)

func rawgitratchetEntry() gatecontract.Entry {
	return gatecontract.Entry{
		Name:   "raw-git-ratchet",
		Kind:   gatecontract.KindScanner,
		Files:  []string{"go/internal/rawgitratchet/"},
		Owners: []gatecontract.Owner{{Pattern: "*", Phase: "build"}},
		Inputs: []gatecontract.Declared{
			{Path: "go/", Class: gatecontract.ClassSubject},
			{Path: "go/" + rawgitratchet.BaselineRelPath, Class: gatecontract.ClassAuthority},
		},
		Stage: gatecontract.StageEnforce,
		Run:   runRawgitratchet,
	}
}

func runRawgitratchet(_ context.Context, in gatecontract.Input, _ gatecontract.Ports) (gatecontract.Verdict, error) {
	sites, baseline, note, err := rawgitratchet.Load(filepath.Join(in.Root, "go"))
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	var notes []string
	if note != "" {
		notes = []string{note}
	}
	return verdictOf(rawgitratchet.Check(sites, baseline), notes...), nil
}
