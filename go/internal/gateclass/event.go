package gateclass

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeI10Violation signalcenter.Code = "HEALING_I10_VIOLATION"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeI10Violation, "the gate classifier flagged a loop cycle diff that changes a kernel surface or weakens a gate (invariant I-10, llm-isolation.md §20.3); the build floor fails and ship refuses the cycle; fields.patterns names each pattern, fields.paths each flagged path")
}

func ViolationEvent(cycle int, runID, phase string, flags []Flag) signalcenter.Event {
	patterns := make([]string, len(flags))
	findings := make([]string, len(flags))
	paths := make([]string, len(flags))
	for i, f := range flags {
		patterns[i], paths[i], findings[i] = string(f.Pattern), f.Path, f.String()
	}
	return signalcenter.Event{
		Cycle: cycle, RunID: runID, Phase: phase, Module: signalcenter.ModuleHealing, Origin: "gateclass.Classify",
		Kind: signalcenter.KindGateRejected, Code: CodeI10Violation, Severity: signalcenter.SeverityWarn,
		Reason: fmt.Sprintf("the gate classifier flagged %d change(s) to a kernel surface or a gate: %s", len(flags), strings.Join(findings, "; ")),
		Fields: map[string]string{"patterns": strings.Join(patterns, ","), "paths": strings.Join(paths, ",")},
	}
}

func (f Flag) String() string {
	if f.Detail == "" {
		return string(f.Pattern) + " " + f.Path
	}
	return string(f.Pattern) + " " + f.Path + " (" + f.Detail + ")"
}
