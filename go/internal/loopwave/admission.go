package loopwave

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type AdmissionWait struct {
	ID        string
	Rank      int
	Cells     []string
	BlockedBy string
}

type AdmissionShadow struct {
	Admitted int
	Waits    []AdmissionWait
}

type AdmissionFn func(wave int, todos []fleet.Todo, text func(id string) string, cfg policy.OrchestratorConfig) (AdmissionShadow, error)

func WithAdmission(fn AdmissionFn) Option {
	return func(e *Engine) { e.admission = fn }
}

func (e *Engine) shadowAdmission(wave, count int, cfg policy.OrchestratorConfig, todos []fleet.Todo, specs []fleet.CycleSpec) func() {
	if e.admission == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(e.stderr, "[loop] WARN: wave %d: admission shadow panicked and was dropped: %v\n", wave, r)
			}
		}()
		e.observeAdmission(wave, count, cfg, todos, specs)
	}()
	return func() { <-done }
}

func (e *Engine) observeAdmission(wave, count int, cfg policy.OrchestratorConfig, todos []fleet.Todo, specs []fleet.CycleSpec) {
	shadow, err := e.admission(wave, todos, e.InboxText(wave), cfg)
	if err != nil {
		fmt.Fprintf(e.stderr, "[loop] WARN: wave %d: admission shadow skipped: %v\n", wave, err)
		return
	}
	fmt.Fprintf(e.stderr, "[loop] wave %d: admission shadow (stage %s): admission launches %d lane(s), the file partition launches %d; %d todo(s) wait for a cell\n",
		wave, cfg.Stage, min(shadow.Admitted, count), len(specs), len(shadow.Waits))
	for _, w := range shadow.Waits {
		lane := laneOf(w.BlockedBy, specs)
		e.warn("Engine.shadowAdmission", wave, CodeOrchAdmitWait,
			fmt.Sprintf("wave %d: admission by cell holds %q (rank %d) behind %q (lane %s)", wave, w.ID, w.Rank, w.BlockedBy, lane),
			map[string]string{"todo": w.ID, "rank": strconv.Itoa(w.Rank), "cells": strings.Join(w.Cells, ","),
				"blocked_by": w.BlockedBy, "blocked_by_lane": lane, "stage": cfg.Stage})
	}
}

func (e *Engine) InboxText(wave int) func(id string) string {
	scan, err := inboxbatch.ScanDir(e.lifecycle().InboxDir)
	if err != nil {
		fmt.Fprintf(e.stderr, "[loop] WARN: wave %d: admission shadow reads no inbox text: %v\n", wave, err)
	}
	text := make(map[string]string, len(scan.Items))
	for _, it := range scan.Items {
		text[it.ID] = it.Title + "\n" + summaryText(it.Summary)
	}
	return func(id string) string { return text[id] }
}

func summaryText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func laneOf(id string, specs []fleet.CycleSpec) string {
	for i, s := range specs {
		if slices.Contains(s.Scope, id) {
			return strconv.Itoa(i + 1)
		}
	}
	return "none"
}

func (e *Engine) AdmissionWired() bool { return e.admission != nil }
