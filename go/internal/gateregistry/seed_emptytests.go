package gateregistry

import (
	"context"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/emptytests"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func emptytestsEntry() gatecontract.Entry {
	return gatecontract.Entry{
		Name:   "empty-tests",
		Kind:   gatecontract.KindScanner,
		Files:  []string{"go/internal/emptytests/"},
		Covers: []string{"./internal/emptytests/..."},
		Owners: []gatecontract.Owner{{Pattern: "*", Phase: "build"}},
		Inputs: []gatecontract.Declared{{Path: "go/", Class: gatecontract.ClassSubject}},
		Stage:  gatecontract.StageEnforce,
		Run:    runEmptytests,
	}
}

func runEmptytests(_ context.Context, in gatecontract.Input) (gatecontract.Verdict, error) {
	found, err := emptytests.Scan(filepath.Join(in.Root, "go"))
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	return verdictOf(emptytests.Check(found)), nil
}
