package core

import (
	"context"
	"io"
)

// Storage reads and writes the .evolve/ filesystem state surface:
// state.json, cycle-state.json, the .lock file. Impls live in
// internal/adapters/storage.
type Storage interface {
	ReadState(ctx context.Context) (State, error)
	WriteState(ctx context.Context, s State) error
	ReadCycleState(ctx context.Context) (CycleState, error)
	WriteCycleState(ctx context.Context, cs CycleState) error
	AcquireLock(ctx context.Context) (release func() error, err error)
}

// Ledger appends to and verifies the .evolve/ledger.jsonl hash chain.
type Ledger interface {
	Append(ctx context.Context, entry LedgerEntry) error
	Verify(ctx context.Context) error
	Iter(ctx context.Context) (LedgerIterator, error)
}

// LedgerIterator yields entries in append order. Close releases the
// underlying file handle.
type LedgerIterator interface {
	Next() (LedgerEntry, bool, error)
	Close() error
}

// Bridge launches an LLM agent via the existing tools/agent-bridge/
// subprocess and parses its JSON output.
type Bridge interface {
	Launch(ctx context.Context, req BridgeRequest) (BridgeResponse, error)
	Probe(ctx context.Context) (BridgeProbe, error)
}

// Sandbox wraps subprocess execution in OS-level isolation
// (sandbox-exec on macOS, bwrap on Linux). Impls live in
// internal/adapters/sandbox.
type Sandbox interface {
	Exec(ctx context.Context, profile SandboxProfile, argv []string, stdin io.Reader, stdout, stderr io.Writer) error
}

// Guard runs a trust-kernel guard. Impls live in internal/guards.
type Guard interface {
	Name() string
	Decide(ctx context.Context, in GuardInput) GuardDecision
}

// Observer streams phase activity into abnormal-events.jsonl + slog.
type Observer interface {
	Watch(ctx context.Context, phase Phase) error
	Stop() error
}

// State mirrors the .evolve/state.json schema (subset used by orchestrator).
// Full field set is round-tripped through encoding/json by the storage
// adapter; this struct exposes only the orchestrator-load-bearing fields.
type State struct {
	LastUpdated     string         `json:"lastUpdated"`
	LastCycleNumber int            `json:"lastCycleNumber"`
	Version         int            `json:"version"`
	CurrentBatch    BatchAccrual   `json:"currentBatch"`
	FailedAt        []FailedRecord `json:"failedApproaches,omitempty"`
	CarryoverTodos  []CarryoverTodo `json:"carryoverTodos,omitempty"`
}

// BatchAccrual tracks per-dispatcher-invocation cost.
type BatchAccrual struct {
	CycleAccruedCostUSD float64 `json:"cycleAccruedCostUSD"`
	GoalHash            string  `json:"goalHash,omitempty"`
}

// FailedRecord captures a non-PASS cycle outcome (for failure suppression).
type FailedRecord struct {
	Cycle           int      `json:"cycle"`
	Verdict         string   `json:"verdict"`
	AuditReportPath string   `json:"auditReportPath"`
	SHA256          string   `json:"sha256"`
	GitHEAD         string   `json:"git_head"`
	TreeStateSHA    string   `json:"treeStateSha"`
	Defects         []string `json:"defects,omitempty"`
	Retrospected    bool     `json:"retrospected"`
}

// CarryoverTodo is operator-queued work surfaced cycle-to-cycle.
type CarryoverTodo struct {
	ID               string `json:"id"`
	Action           string `json:"action"`
	Priority         string `json:"priority"`
	FirstSeenCycle   int    `json:"first_seen_cycle"`
	CyclesUnpicked   int    `json:"cycles_unpicked"`
}

// CycleState mirrors .evolve/cycle-state.json (transient per-cycle).
type CycleState struct {
	CycleID          int      `json:"cycle_id"`
	Phase            string   `json:"phase"`
	StartedAt        string   `json:"started_at"`
	PhaseStartedAt   string   `json:"phase_started_at"`
	ActiveAgent      string   `json:"active_agent,omitempty"`
	ActiveWorktree   string   `json:"active_worktree,omitempty"`
	CompletedPhases  []string `json:"completed_phases,omitempty"`
	WorkspacePath    string   `json:"workspace_path"`
	IntentRequired   bool     `json:"intent_required"`
}

// LedgerEntry is one .jsonl line; fields match the v8.37+ shape
// observed in .evolve/ledger.jsonl. Extra fields per kind are carried
// in Extra so future agent kinds don't require schema migration.
type LedgerEntry struct {
	TS              string         `json:"ts"`
	Cycle           int            `json:"cycle"`
	Role            string         `json:"role"`
	Kind            string         `json:"kind"`
	Model           string         `json:"model,omitempty"`
	ExitCode        int            `json:"exit_code"`
	DurationS       string         `json:"duration_s,omitempty"`
	ArtifactPath    string         `json:"artifact_path,omitempty"`
	ArtifactSHA256  string         `json:"artifact_sha256,omitempty"`
	ChallengeToken  string         `json:"challenge_token,omitempty"`
	GitHEAD         string         `json:"git_head,omitempty"`
	TreeStateSHA    string         `json:"tree_state_sha,omitempty"`
	EntrySeq        int            `json:"entry_seq"`
	PrevHash        string         `json:"prev_hash"`
	WorkerCount     int            `json:"worker_count,omitempty"`
	Workers         []string       `json:"workers,omitempty"`
	Extra           map[string]any `json:"-"`
}

// BridgeRequest is the input to Bridge.Launch. Field shape mirrors the
// flag surface of `tools/agent-bridge/bin/bridge launch`. The adapter
// writes Prompt to a file under Workspace before invoking the bridge
// subprocess (callers don't manage tmp-file lifecycle).
type BridgeRequest struct {
	CLI          string            `json:"cli"`            // claude-p | claude-tmux | codex | agy
	Profile      string            `json:"profile"`        // absolute path to .evolve/profiles/<name>.json
	Model        string            `json:"model"`          // haiku | sonnet | opus | auto | gpt-* | gemini-*
	Prompt       string            `json:"prompt"`         // prompt body; adapter materializes as a file
	Workspace    string            `json:"workspace"`      // absolute path; bridge writes outputs here
	Worktree     string            `json:"worktree,omitempty"`
	StdoutLog    string            `json:"stdout_log,omitempty"`
	StderrLog    string            `json:"stderr_log,omitempty"`
	ArtifactPath string            `json:"artifact_path,omitempty"` // adapter requires non-empty
	Agent        string            `json:"agent,omitempty"`         // role label
	Cycle        int               `json:"cycle,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	ExtraFlags   []string          `json:"extra_flags,omitempty"` // pass-through to bridge
}

// BridgeResponse is the bridge's JSON-parsed reply.
type BridgeResponse struct {
	ExitCode    int        `json:"exit_code"`
	Stdout      string     `json:"stdout"`
	Stderr      string     `json:"stderr"`
	CostUSD     float64    `json:"cost_usd"`
	Tokens      TokenUsage `json:"tokens"`
	DurationMS  int64      `json:"duration_ms"`
}

// BridgeProbe is what bridge reports about its environment + CLIs.
type BridgeProbe struct {
	Version string            `json:"version"`
	CLIs    map[string]string `json:"clis"` // cli name → tier (full/degraded/none)
}

// SandboxProfile selects an OS-level isolation policy for Sandbox.Exec.
type SandboxProfile struct {
	Name         string   // matches .evolve/profiles/<agent>.json:sandbox_profile
	ReadOnlyRepo bool     // Auditor/Evaluator must NOT write to repo
	AllowedDirs  []string // additional writable paths beyond /tmp + worktree
}

// GuardInput is the typed input to a guard's Decide() method.
type GuardInput struct {
	ToolName     string         // "Bash" | "Edit" | "Write" | "Agent" | "WebSearch" | …
	ToolInput    map[string]any // raw stdin JSON tool_input
	CWD          string
	CycleStatePath string       // optional; defaults to <CWD>/.evolve/cycle-state.json
}

// GuardDecision is what a guard returns. Allow=true → exit 0; Allow=false → exit 2.
// Reason is logged to .evolve/guards.log and written to stderr.
type GuardDecision struct {
	Allow  bool
	Reason string
}
