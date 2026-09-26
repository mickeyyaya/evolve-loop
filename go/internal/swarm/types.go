// Package swarm splits one phase's task across N parallel LLM-CLI workers and fans their results back in.
// Writers need disjoint file ownership and merge through a serialized train; readers may overlap and are synthesized.
// See docs/architecture/packages/internal-swarm.md.
package swarm

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"

// Mode selects writer handling (strict disjointness, merge train) or reader handling (overlap allowed, synthesis).
type Mode string

const (
	// ModeWriter requires disjoint file ownership and collapses to N=1 otherwise.
	ModeWriter Mode = "writer"
	// ModeReader allows overlapping focus regions and never collapses for overlap.
	ModeReader Mode = "reader"
)

// WorkerSpec is one worker's assignment within a SwarmPlan.
type WorkerSpec struct {
	WorkerID string `json:"worker_id"`
	CLI      string `json:"cli"`
	Model    string `json:"model"`
	Profile  string `json:"profile,omitempty"`
	// Branch is a writer's dev branch; empty for readers.
	Branch string `json:"branch,omitempty"`
	// TargetFiles is file ownership for writers (disjoint across workers) or focus regions for readers.
	TargetFiles []string `json:"target_files,omitempty"`
	// DependsOn names the workers that must merge before this one, which orders the merge train.
	DependsOn []string `json:"depends_on,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	// Acceptance gates a writer's merge-train step, or is reported in a reader's summary.
	Acceptance []string `json:"acceptance,omitempty"`
}

// SwarmPlan is the planner's partition of one phase's task, parsed from swarm-plan.md.
type SwarmPlan struct {
	TaskID string `json:"task_id,omitempty"`
	Mode   Mode   `json:"mode"`
	// Partitionable is the planner's verdict; for writers it is true only for a fully disjoint split.
	Partitionable bool   `json:"partitionable"`
	Rationale     string `json:"rationale,omitempty"`
	// IntegrationBranch is the branch writer dev branches merge into.
	IntegrationBranch string       `json:"integration_branch,omitempty"`
	Workers           []WorkerSpec `json:"workers,omitempty"`
}

type planEnvelope struct {
	SwarmPlan SwarmPlan `json:"swarm_plan"`
}

// IsFallback reports whether the plan collapses to one worker: declared non-partitionable, or fewer than two workers.
func (p SwarmPlan) IsFallback() bool {
	return !p.Partitionable || len(p.Workers) < 2
}

// WorkerResult is the observable outcome of one dispatched worker.
type WorkerResult struct {
	WorkerID string
	Agent    string
	Branch   string // writers: the dev branch the worker committed to
	Worktree string
	// ArtifactPath is the report the dispatcher told this worker to write.
	ArtifactPath string
	ExitCode     int
	CostUSD      float64
	Tokens       cyclestate.TokenUsage
	Err          error // launch or transport failure, as opposed to a clean non-zero exit
}

// OK reports a worker run with no transport error and a zero exit code.
func (r WorkerResult) OK() bool { return r.Err == nil && r.ExitCode == 0 }

// SwarmResult is the reduced outcome of one swarm dispatch, the input to the orchestrator's N→1 aggregation.
type SwarmResult struct {
	Mode              Mode
	IntegrationBranch string
	// IntegrationWorktree is the writers' integration worktree, where the merge train runs; empty for readers.
	IntegrationWorktree string
	Workers             []WorkerResult
	// MergeOrder is the validated serialized merge order for writers; nil for readers.
	MergeOrder []string
}

// AllOK reports whether there was at least one worker and every worker succeeded.
func (s SwarmResult) AllOK() bool {
	for _, w := range s.Workers {
		if !w.OK() {
			return false
		}
	}
	return len(s.Workers) > 0
}

// TotalCostUSD sums per-worker cost for the single aggregated ledger entry.
func (s SwarmResult) TotalCostUSD() float64 {
	var sum float64
	for _, w := range s.Workers {
		sum += w.CostUSD
	}
	return sum
}

// TotalTokens sums per-worker token usage field by field, so counts survive the N→1 ledger merge.
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

// Conflict records a file claimed by more than one writer worker.
type Conflict struct {
	File    string
	Workers []string
}

// ValidationResult is Validate's verdict over a SwarmPlan.
type ValidationResult struct {
	OK bool // safe to dispatch as a swarm
	// Collapse means fall back to one worker: a declared fallback or an unrepairable writer overlap.
	Collapse bool
	Reason   string
	// Conflicts are writer file-ownership overlaps; empty for readers.
	Conflicts []Conflict
	// MergeOrder is the writers' serialized order from the depends_on DAG; nil for readers.
	MergeOrder []string
}
