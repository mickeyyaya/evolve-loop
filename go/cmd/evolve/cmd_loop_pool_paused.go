package main

import (
	"fmt"
	"io"
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
	itemFiles func(id string) ([]string, bool)
	warn      io.Writer
}

func (pp pausedPlan) order(triage []fleet.Todo) []fleet.Todo {
	var head []fleet.Todo
	rest := slices.Clone(triage)
	for _, p := range pp.paused {
		if !pp.ready(p) {
			continue
		}
		if lead, remaining, ok := pp.lead(p, rest); ok {
			head, rest = append(head, lead), remaining
		}
	}
	return append(head, rest...)
}

func (pp pausedPlan) ready(p core.PausedScope) bool {
	ready, err := checkpoint.ResumeReady(p.AutoAttempts, p.AutoMaxAttempts, p.QuotaResetAt, pp.now)
	if err != nil {
		fmt.Fprintf(pp.warn, "[pool] WARN: paused cycle %d: %v\n", p.Cycle, err)
	}
	return ready
}

func (pp pausedPlan) lead(p core.PausedScope, triage []fleet.Todo) (fleet.Todo, []fleet.Todo, bool) {
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
			files, located := pp.itemFiles(todo)
			found = found || located
			lead.Files = append(lead.Files, files...)
		}
	}
	slices.Sort(lead.Files)
	lead.Files = slices.Compact(lead.Files)
	return lead, rest, found
}

func inboxItemFiles(evolveDir string) func(id string) ([]string, bool) {
	inbox := filepath.Join(evolveDir, "inbox")
	return func(id string) ([]string, bool) {
		loc, err := inboxmover.Locate(inbox, id)
		if err != nil {
			return nil, false
		}
		item, _, err := inboxbatch.LoadFile(loc.Path)
		if err != nil {
			return nil, false
		}
		return item.DeclaredPaths(), true
	}
}
