package citelemetry

import "testing"

func TestExportedSymbols_AreNamedAndWorkThroughTheEntryPoint(t *testing.T) {
	rep := fixtureReport(t)
	var (
		g1      G1       = rep.G1
		step    PathStep = rep.CriticalPath[0]
		pkg     Package  = rep.Packages[0]
		usage   *Rusage  = pkg.Rusage
		sampler *Sampler = pkg.Sampler
	)
	if g1.MacosJobS != 1350 || step.Level != "L5" || usage.UserS != 40 || sampler.Samples != 2 {
		t.Errorf("fixture report = %+v", rep)
	}
	if md := rep.Markdown(); md == "" {
		t.Errorf("Markdown is empty")
	}
}
