package main

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const admissionGraphTimeout = 30 * time.Second

func admissionShadow(ctx context.Context, goDir string, run sysexec.RunFunc) loopwave.AdmissionFn {
	return func(todos []fleet.Todo, text func(string) string, cfg policy.OrchestratorConfig) (loopwave.AdmissionShadow, error) {
		bounded, cancel := context.WithTimeout(ctx, admissionGraphTimeout)
		defer cancel()
		graph, err := admission.LoadModuleGraph(bounded, run, goDir, cfg.HubMinImporters)
		if err != nil {
			return loopwave.AdmissionShadow{}, err
		}
		admitted, waiting := admission.Admitter{Graph: graph, Text: text}.Admit(nil, todos, cfg.ScanK)
		shadow := loopwave.AdmissionShadow{Admitted: len(admitted)}
		for _, w := range waiting {
			shadow.Waits = append(shadow.Waits, loopwave.AdmissionWait(w))
		}
		return shadow, nil
	}
}
