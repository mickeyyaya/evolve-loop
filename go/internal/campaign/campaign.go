// Package campaign is the multi-cycle campaign planner.
package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const (
	MaxCycles    = 50
	MaxWaveWidth = 10
)

type Plan struct {
	Version  int          `json:"version"`
	Goal     string       `json:"goal"`
	Research Research     `json:"research,omitempty"`
	Cycles   []fleet.Todo `json:"cycles"`
}

type Research struct {
	Summary   string     `json:"summary,omitempty"`
	Citations []Citation `json:"citations,omitempty"`
}

type Citation struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Note  string `json:"note,omitempty"`
}

func PlanFromUnifiedCommitment(c inboxbatch.UnifiedCommitment, items []inboxbatch.Item) (*Plan, error) {
	if err := c.Validate(items); err != nil {
		return nil, err
	}
	byID := make(map[string]inboxbatch.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	plan := &Plan{
		Version: 1,
		Goal:    c.RootCauseHypothesis,
		Research: Research{
			Summary: c.SharedSeam,
		},
	}
	for _, member := range c.Members {
		item := byID[member.ID]
		contract := strings.Join(item.Acceptance, "\n")
		if contract == "" {
			contract = member.Evidence
		}
		plan.Research.Citations = append(plan.Research.Citations, Citation{Title: member.ID, Note: member.Evidence})
		plan.Cycles = append(plan.Cycles, fleet.Todo{
			ID:             item.ID,
			Files:          append([]string(nil), item.Files...),
			DependsOn:      append([]string(nil), item.Deps...),
			OutputContract: contract,
		})
	}
	if err := plan.Verify(); err != nil {
		return nil, err
	}
	return plan, nil
}

func Load(data []byte) (*Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("campaign: parse plan: %w", err)
	}
	return &p, nil
}

func LoadFile(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("campaign: read plan: %w", err)
	}
	return Load(data)
}

func (p *Plan) Verify() error {
	if p.Version <= 0 {
		return fmt.Errorf("campaign: plan version must be >= 1")
	}
	if strings.TrimSpace(p.Goal) == "" {
		return fmt.Errorf("campaign: plan goal is empty")
	}
	if len(p.Cycles) == 0 {
		return fmt.Errorf("campaign: plan has no cycles")
	}
	if len(p.Cycles) > MaxCycles {
		return fmt.Errorf("campaign: %d cycles exceeds max %d", len(p.Cycles), MaxCycles)
	}
	seen := make(map[string]bool, len(p.Cycles))
	for _, c := range p.Cycles {
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("campaign: a cycle has an empty id")
		}
		if seen[c.ID] {
			return fmt.Errorf("campaign: duplicate cycle id %q", c.ID)
		}
		if strings.ContainsAny(c.ID, ",") {
			return fmt.Errorf("campaign: cycle id %q contains invalid char", c.ID)
		}
		seen[c.ID] = true
	}
	waves, err := fleet.PlanWaves(p.Cycles)
	if err != nil {
		return fmt.Errorf("campaign: invalid cycle DAG: %w", err)
	}
	for i, wave := range waves {
		if len(wave) > MaxWaveWidth {
			return fmt.Errorf("campaign: wave %d width %d exceeds max %d", i+1, len(wave), MaxWaveWidth)
		}
	}
	return nil
}

func (p *Plan) Waves() ([][]fleet.CycleSpec, error) {
	return fleet.PlanWaves(p.Cycles)
}

func Diff(old, updated *Plan) string {
	oldByID := indexByID(old)
	newByID := indexByID(updated)
	var added, removed, modified []string
	for id := range newByID {
		if _, ok := oldByID[id]; !ok {
			added = append(added, id)
		}
	}
	for id, oc := range oldByID {
		nc, ok := newByID[id]
		if !ok {
			removed = append(removed, id)
			continue
		}
		if !sameCycle(oc, nc) {
			modified = append(modified, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(modified)
	return fmt.Sprintf("added: %v; removed: %v; modified: %v", added, removed, modified)
}

func indexByID(p *Plan) map[string]fleet.Todo {
	m := make(map[string]fleet.Todo, len(p.Cycles))
	for _, c := range p.Cycles {
		m[c.ID] = c
	}
	return m
}

func sameCycle(a, b fleet.Todo) bool {
	return eqStrings(a.Files, b.Files) &&
		eqStrings(a.DependsOn, b.DependsOn) &&
		a.Priority == b.Priority &&
		a.OutputContract == b.OutputContract &&
		eqStrings(a.ToolScope, b.ToolScope)
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
