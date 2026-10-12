package core

import (
	"io"
	"os"
	"testing"
)

func TestStderrWriters_EveryConstructorDefaultsToOsStderr(t *testing.T) {
	t.Parallel()
	cases := map[string]io.Writer{
		"NewOrchestrator":                 NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).errOut(),
		"NewOrchestrator with a nil sink": NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(nil)).errOut(),
		"a nil orchestrator":              (*Orchestrator)(nil).errOut(),
		"an orchestrator literal":         (&Orchestrator{}).errOut(),
		"ReviewInput":                     ReviewInput{}.errOut(),
		"ReviewInputFor":                  ReviewInputFor(CycleState{}, PhaseBuild, "").errOut(),
		"leakRecovery":                    leakRecovery{}.errOut(),
	}
	for name, got := range cases {
		if got != io.Writer(os.Stderr) {
			t.Errorf("%s writes to %T, want os.Stderr by default", name, got)
		}
	}
}

func TestStderrWriters_AnInjectedWriterIsTheDestination(t *testing.T) {
	t.Parallel()
	var sink injectedStderr
	sinkWriter := io.Writer(&sink)
	cases := map[string]io.Writer{
		"WithStderr":   NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&sink)).errOut(),
		"ReviewInput":  ReviewInput{Stderr: &sink}.errOut(),
		"leakRecovery": leakRecovery{warn: &sink}.errOut(),
	}
	for name, got := range cases {
		if got != sinkWriter {
			t.Errorf("%s writes to %T, want the injected writer", name, got)
		}
	}
}
