package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func admissionShadow(graphOf func(oc policy.OrchestratorConfig, wave int) (*admission.ModuleGraph, error)) loopwave.AdmissionFn {
	return func(wave int, todos []fleet.Todo, text func(string) string, cfg policy.OrchestratorConfig) (loopwave.AdmissionShadow, error) {
		graph, err := graphOf(cfg, wave)
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
