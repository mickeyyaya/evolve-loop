// Package swarm splits one phase's task across N parallel LLM-CLI workers and fans their results back in.
// Writers need disjoint file ownership and merge through a serialized train; readers may overlap and are synthesized.
// See docs/architecture/packages/internal-swarm.md.
package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"

type Mode string

const (
	ModeWriter Mode = "writer"
	ModeReader Mode = "reader"
)

type WorkerSpec struct {
	WorkerID    string   `json:"worker_id"`
	CLI         string   `json:"cli"`
	Model       string   `json:"model"`
	Profile     string   `json:"profile,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	TargetFiles []string `json:"target_files,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Acceptance  []string `json:"acceptance,omitempty"`
}

type SwarmPlan struct {
	TaskID            string       `json:"task_id,omitempty"`
	Mode              Mode         `json:"mode"`
	Partitionable     bool         `json:"partitionable"`
	Rationale         string       `json:"rationale,omitempty"`
	IntegrationBranch string       `json:"integration_branch,omitempty"`
	Workers           []WorkerSpec `json:"workers,omitempty"`
}

type planEnvelope struct {
	SwarmPlan SwarmPlan `json:"swarm_plan"`
}

func (p SwarmPlan) IsFallback() bool {
	return !p.Partitionable || len(p.Workers) < 2
}

type WorkerResult struct {
	WorkerID     string
	Agent        string
	Branch       string
	Worktree     string
	ArtifactPath string
	ExitCode     int
	CostUSD      float64
	Tokens       cyclestate.TokenUsage
	Err          error
}

func (r WorkerResult) OK() bool { return r.Err == nil && r.ExitCode == 0 }

type SwarmResult struct {
	Mode                Mode
	IntegrationBranch   string
	IntegrationWorktree string
	Workers             []WorkerResult
	MergeOrder          []string
}

func (s SwarmResult) AllOK() bool {
	for _, w := range s.Workers {
		if !w.OK() {
			return false
		}
	}
	return len(s.Workers) > 0
}

func (s SwarmResult) TotalCostUSD() float64 {
	var sum float64
	for _, w := range s.Workers {
		sum += w.CostUSD
	}
	return sum
}

func (s SwarmResult) TotalTokens() cyclestate.TokenUsage {
	var sum cyclestate.TokenUsage
	for _, w := range s.Workers {
		sum.Input += w.Tokens.Input
		sum.Output += w.Tokens.Output
		sum.CacheRead += w.Tokens.CacheRead
		sum.CacheWrite += w.Tokens.CacheWrite
	}
	return sum
}

type Conflict struct {
	File    string
	Workers []string
}

type ValidationResult struct {
	OK         bool
	Collapse   bool
	Reason     string
	Conflicts  []Conflict
	MergeOrder []string
}
