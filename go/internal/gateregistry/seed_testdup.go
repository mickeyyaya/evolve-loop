package gateregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

func testdupEntry() gatecontract.Entry {
	return gatecontract.Entry{
		Name:  "testdup-ratchet",
		Kind:  gatecontract.KindScanner,
		Files: []string{"go/internal/testdup/"},
		Owners: []gatecontract.Owner{
			{Pattern: "go/acs/cycle*/", Phase: "tdd"},
			{Pattern: "*", Phase: "build"},
		},
		Inputs: []gatecontract.Declared{
			{Path: "go/", Class: gatecontract.ClassSubject},
			{Path: "go/" + testdup.BaselineRelPath, Class: gatecontract.ClassOpenSeam},
		},
		Stage: gatecontract.StageEnforce,
		Run:   runTestdup,
	}
}

func runTestdup(_ context.Context, in gatecontract.Input, _ gatecontract.Ports) (gatecontract.Verdict, error) {
	module := filepath.Join(in.Root, "go")
	raw, err := os.ReadFile(filepath.Join(module, testdup.BaselineRelPath))
	if err != nil {
		return gatecontract.Verdict{}, fmt.Errorf("testdup-ratchet: read baseline: %w", err)
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	report, err := testdup.Scan(module, baseline.Date)
	if err != nil {
		return gatecontract.Verdict{}, err
	}
	return verdictOf(testdup.Check(report, baseline)), nil
}
