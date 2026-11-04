package main

import (
	"errors"
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
	adoptable func(p core.PausedScope) error
	itemFiles func(p core.PausedScope) ([]string, error)
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
		pp.warnf("[pool] WARN: paused cycle %d: %v\n", p.Cycle, err)
	}
	return ready
}

func (pp pausedPlan) warnf(format string, args ...any) {
	if pp.warn != nil {
		fmt.Fprintf(pp.warn, format, args...)
	}
}

func (pp pausedPlan) lead(p core.PausedScope, triage []fleet.Todo) (fleet.Todo, []fleet.Todo, bool) {
	id := strings.Join(p.Scope, ",")
	if err := pp.adoptable(p); err != nil {
		pp.warnf("[pool] WARN: paused cycle %d: scope %s is not put first: %v\n", p.Cycle, id, err)
		return fleet.Todo{}, triage, false
	}
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
		files, err := pp.itemFiles(p)
		if err != nil {
			pp.warnf("[pool] WARN: paused cycle %d: scope %s keeps its place without some file claims: %v\n", p.Cycle, id, err)
		}
		lead.Files = files
	}
	slices.Sort(lead.Files)
	lead.Files = slices.Compact(lead.Files)
	return lead, rest, true
}

func inboxAdoptable(evolveDir string) func(p core.PausedScope) error {
	inbox := filepath.Join(evolveDir, "inbox")
	return func(p core.PausedScope) error {
		if len(p.Scope) == 0 {
			return errors.New("the scope has no item")
		}
		return scopeHold{inbox: inbox, scope: p.Scope}.adoptableBy(p.Cycle)
	}
}

func inboxItemFiles(evolveDir string) func(p core.PausedScope) ([]string, error) {
	inbox := filepath.Join(evolveDir, "inbox")
	return func(p core.PausedScope) ([]string, error) {
		var files []string
		var lost []error
		for _, id := range p.Scope {
			declared, err := declaredFiles(inbox, id)
			files = append(files, declared...)
			if err != nil {
				lost = append(lost, err)
			}
		}
		return files, errors.Join(lost...)
	}
}

func declaredFiles(inbox, id string) ([]string, error) {
	loc, err := inboxmover.Locate(inbox, id)
	if err != nil {
		return nil, fmt.Errorf("the files of %s are unknown: %w", id, err)
	}
	item, _, err := inboxbatch.LoadFile(loc.Path)
	if err != nil {
		return nil, fmt.Errorf("the files of %s are unknown: %w", id, err)
	}
	return item.DeclaredPaths(), nil
}
