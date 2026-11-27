package gateregistry

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/testrefs"
)

func testrefsEntry() gatecontract.Entry {
	return gatecontract.Entry{
		Name:   "named-tests",
		Kind:   gatecontract.KindScanner,
		Files:  []string{"go/internal/testrefs/testrefs.go", "go/internal/testrefs/testrefs_test.go"},
		Owners: []gatecontract.Owner{{Pattern: "*", Phase: "build"}},
		Inputs: []gatecontract.Declared{
			{Path: "go/", Class: gatecontract.ClassSubject},
			{Path: "docs/", Class: gatecontract.ClassSubject},
			{Path: "skills/", Class: gatecontract.ClassSubject},
			{Path: "agents/", Class: gatecontract.ClassSubject},
			{Path: "go/" + testrefs.BaselineRelPath, Class: gatecontract.ClassVerified},
		},
		Stage: gatecontract.StageEnforce,
		Run:   runTestrefs,
	}
}

func runTestrefs(_ context.Context, in gatecontract.Input) (gatecontract.Verdict, error) {
	problems, err := testrefs.Scan(in.Root)
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	return gatecontract.Verdict{Reds: problems}, nil
}
