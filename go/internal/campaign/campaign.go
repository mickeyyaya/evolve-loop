// Package campaign is the multi-cycle campaign planner. It loads the
// preliminary-study phase's campaign-plan.json, deterministically VERIFIES it
// (the trust boundary — versioned, unique cycles, an acyclic depends_on DAG with
// every dependency resolvable), and turns it into dependency-ordered execution
// waves via fleet.PlanWaves.
//
// Per the 2026 finding that intrinsic LLM self-critique is unreliable for plan
// validation, verification here is PURELY DETERMINISTIC; the LLM plan-reviewer is
// advisory and lives in the driver, never the trust boundary. The per-cycle
// integrity floor (build∧audit∧tdd before ship) is enforced downstream by each
// launched cycle's pipeline — a campaign plan has no field to weaken it.
package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

const (
	MaxCycles    = 50
	MaxWaveWidth = 10
)

// Plan is the decomposition the preliminary-study phase emits: a versioned,
// dependency-annotated backlog of cycle-tasks plus the research that grounds it.
type Plan struct {
	Version  int          `json:"version"`
	Goal     string       `json:"goal"`
	Research Research     `json:"research,omitempty"`
	Cycles   []fleet.Todo `json:"cycles"`
}

// Research records the planning evidence (cited sources) so the decomposition is
// grounded, not vibe — surfaced in the human-readable render before approval.
type Research struct {
	Summary   string     `json:"summary,omitempty"`
	Citations []Citation `json:"citations,omitempty"`
}

// Citation is one grounding source for the decomposition strategy.
type Citation struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Load parses campaign-plan.json, rejecting unknown fields so schema drift fails
// loud rather than silently dropping data.
func Load(data []byte) (*Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("campaign: parse plan: %w", err)
	}
	return &p, nil
}

// LoadFile reads and parses a campaign-plan.json file.
func LoadFile(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("campaign: read plan: %w", err)
	}
	return Load(data)
}

// Verify is the deterministic trust boundary: a plan executes only if it has a
// version + goal, at least one cycle, unique non-empty cycle ids, and a
// depends_on DAG that is acyclic with every dependency resolvable (no dangling
// refs). Returns the first violation.
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
		seen[c.ID] = true
	}
	// DAG validity (acyclic + every depends_on resolvable) — reuse the wave planner.
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

// Waves returns the dependency-ordered execution waves (each wave's cycles are
// file-disjoint and run concurrently; waves run in order). Verify should pass first.
func (p *Plan) Waves() ([][]fleet.CycleSpec, error) {
	return fleet.PlanWaves(p.Cycles)
}

// Diff summarizes cycle-level changes between two plan versions for the
// approve→replan loop, deterministically (added/removed/modified cycle ids).
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

// sameCycle reports whether two cycle-tasks are equivalent for diff purposes.
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
