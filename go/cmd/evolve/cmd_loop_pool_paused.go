package main

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

type pausedPlan struct {
	paused    []core.PausedScope
	now       time.Time
	itemFiles func(id string) []string
}

func (pp pausedPlan) order(triage []fleet.Todo) []fleet.Todo {
	var head []fleet.Todo
	rest := slices.Clone(triage)
	for _, p := range pp.paused {
		if !pp.ready(p) {
			continue
		}
		var lead fleet.Todo
		lead, rest = pp.lead(p, rest)
		head = append(head, lead)
	}
	return append(head, rest...)
}

func (pp pausedPlan) ready(p core.PausedScope) bool {
	return checkpoint.ResumeReady(p.AutoAttempts, p.AutoMaxAttempts, p.QuotaResetAt, pp.now)
}

func (pp pausedPlan) lead(p core.PausedScope, triage []fleet.Todo) (fleet.Todo, []fleet.Todo) {
	id := strings.Join(p.Scope, ",")
	lead, found := fleet.Todo{ID: id}, false
	var rest []fleet.Todo
	for _, td := range triage {
		if td.ID != id && !slices.Contains(p.Scope, td.ID) {
			rest = append(rest, td)
			continue
		}
		if !found {
			lead, found = td, true
			lead.ID, lead.Files = id, nil
		}
		lead.Files = append(lead.Files, td.Files...)
	}
	if !found {
		for _, todo := range p.Scope {
			lead.Files = append(lead.Files, pp.itemFiles(todo)...)
		}
	}
	slices.Sort(lead.Files)
	lead.Files = slices.Compact(lead.Files)
	return lead, rest
}

func inboxItemFiles(evolveDir string) func(id string) []string {
	inbox := filepath.Join(evolveDir, "inbox")
	return func(id string) []string {
		loc, err := inboxmover.Locate(inbox, id)
		if err != nil {
			return nil
		}
		item, _, err := inboxbatch.LoadFile(loc.Path)
		if err != nil {
			return nil
		}
		return item.DeclaredPaths()
	}
}
