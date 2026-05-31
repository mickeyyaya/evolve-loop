// Package swarm implements the multi-tmux-LLM-CLI subagent swarm harness: a
// reusable primitive that lets one orchestration phase dispatch N heterogeneous
// workers (each its own CLI/model) that collaborate on a partitioned task.
//
// This file defines the pure value types shared across the package. The
// partition validator (partition.go) and topological merge-order (topo.go) are
// pure functions over these types — no I/O, no core dependency — so they are
// exhaustively table-testable. Dispatch, registry, merge-train, and reaper
// (later increments) build on top.
//
// The central rule is the WRITER/READER asymmetry (see ADR-0032):
//   - WRITER swarm (e.g. build): workers WRITE code. Partitions MUST be
//     completely disjoint by file ownership; a non-disjoint plan is REJECTED and
//     the phase falls back to a single writer (N=1). Fan-in is a serialized git
//     merge-train.
//   - READER swarm (e.g. scout/audit/research): workers only READ. Partitions
//     are best-effort by investigative aspect; OVERLAP IS ALLOWED (read overlap
//     wastes tokens, never corrupts). Fan-in is summary synthesis (no git).
package swarm

// Mode distinguishes the two swarm postures. It is the single switch that
// selects strict-disjoint (writer) vs lenient-overlap (reader) validation and
// merge-train vs synthesis fan-in.
type Mode string

const (
	// ModeWriter requires disjoint file ownership; rejects to N=1 otherwise.
	ModeWriter Mode = "writer"
	// ModeReader allows overlapping focus regions; never rejects for overlap.
	ModeReader Mode = "reader"
)

// WorkerSpec is one worker's assignment within a SwarmPlan. It maps onto a
// core.BridgeRequest at dispatch time (CLI/Model/Profile/Branch + a per-worker
// workspace and the "<phase>-w<i>" agent name).
type WorkerSpec struct {
	WorkerID string `json:"worker_id"`
	CLI      string `json:"cli"`
	Model    string `json:"model"`
	Profile  string `json:"profile,omitempty"`
	// Branch is the worker's dev branch (writers only; derived as
	// cycle-<N>-w<i>). Empty for readers, which need no branch/worktree.
	Branch string `json:"branch,omitempty"`
	// TargetFiles is the worker's file ownership (writers: MUST be disjoint
	// across workers) or focus regions (readers: overlap allowed).
	TargetFiles []string `json:"target_files,omitempty"`
	// DependsOn lists worker IDs that must already be merged into the
	// integration branch before this worker merges — the inter-worker DAG that
	// determines merge-train order. Writers only.
	DependsOn []string `json:"depends_on,omitempty"`
	// Scope is a one-line statement of what this worker owns.
	Scope string `json:"scope,omitempty"`
	// Acceptance lists this worker's testable done-criteria, gating its
	// merge-train step (writers) or reported in its summary (readers).
	Acceptance []string `json:"acceptance,omitempty"`
}

// SwarmPlan is the planner's output: a partition of one phase's task into N
// worker assignments. It is parsed from the swarm-plan.md JSON block.
type SwarmPlan struct {
	TaskID string `json:"task_id,omitempty"`
	Mode   Mode   `json:"mode"`
	// Partitionable is the planner's own verdict. For writers it is true ONLY
	// when the planner achieved a completely independent, disjoint split;
	// otherwise the planner sets it false (strongly biased toward N=1).
	Partitionable bool `json:"partitionable"`
	// Rationale explains why this partition (or why not).
	Rationale string `json:"rationale,omitempty"`
	// IntegrationBranch is the shared branch worker dev branches merge into
	// (writers). Conventionally cycle-<N>-integration.
	IntegrationBranch string       `json:"integration_branch,omitempty"`
	Workers           []WorkerSpec `json:"workers,omitempty"`
}

// planEnvelope is the on-disk wrapper: the planner emits {"swarm_plan": {...}}.
type planEnvelope struct {
	SwarmPlan SwarmPlan `json:"swarm_plan"`
}

// IsFallback reports whether this plan should collapse to a single worker
// (N=1, today's single-writer path): the planner declared it non-partitionable,
// or there are fewer than two workers to dispatch.
func (p SwarmPlan) IsFallback() bool {
	return !p.Partitionable || len(p.Workers) < 2
}

// Conflict records a file claimed by more than one worker (a writer-mode
// disjointness violation).
type Conflict struct {
	File    string
	Workers []string
}

// ValidationResult is the pure validator's verdict over a SwarmPlan.
type ValidationResult struct {
	// OK is true when the plan is safe to dispatch as a swarm.
	OK bool
	// Collapse is true when the orchestrator must fall back to N=1 (single
	// worker) instead of swarming — either the plan is a declared fallback, or
	// a writer plan had an unrepairable overlap.
	Collapse bool
	// Reason is a human-readable explanation (logged; shown in shadow mode).
	Reason string
	// Conflicts are writer-mode file-ownership overlaps (empty for readers).
	Conflicts []Conflict
	// MergeOrder is the serialized worker merge order from the depends_on DAG
	// (writers). Nil for readers (synthesis fan-in has no order).
	MergeOrder []string
}
