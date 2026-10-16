package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type Storage interface {
	ReadState(ctx context.Context) (State, error)
	WriteState(ctx context.Context, s State) error
	ReadCycleState(ctx context.Context) (CycleState, error)
	WriteCycleState(ctx context.Context, cs CycleState) error
	AcquireLock(ctx context.Context) (release func() error, err error)
}

type Ledger interface {
	Append(ctx context.Context, entry LedgerEntry) error
	Verify(ctx context.Context) error
	Iter(ctx context.Context) (LedgerIterator, error)
}

type LedgerIterator interface {
	Next() (LedgerEntry, bool, error)
	Close() error
}

type Bridge interface {
	Launch(ctx context.Context, req BridgeRequest) (BridgeResponse, error)
	Probe(ctx context.Context) (BridgeProbe, error)
}

type Guard interface {
	Name() string
	Decide(ctx context.Context, in GuardInput) GuardDecision
}

type (
	State                 = cyclestate.State
	CycleState            = cyclestate.CycleState
	BatchAccrual          = cyclestate.BatchAccrual
	FailedRecord          = cyclestate.FailedRecord
	CarryoverTodo         = cyclestate.CarryoverTodo
	TriageThroughputEntry = cyclestate.TriageThroughputEntry
)

type LedgerEntry struct {
	TS              string   `json:"ts"`
	Cycle           int      `json:"cycle"`
	CycleLabel      string   `json:"cycle_label,omitempty"`
	Role            string   `json:"role"`
	Kind            string   `json:"kind"`
	Model           string   `json:"model,omitempty"`
	ExitCode        int      `json:"exit_code"`
	DurationS       string   `json:"duration_s,omitempty"`
	ArtifactPath    string   `json:"artifact_path,omitempty"`
	ArtifactSHA256  string   `json:"artifact_sha256,omitempty"`
	ChallengeToken  string   `json:"challenge_token,omitempty"`
	GitHEAD         string   `json:"git_head,omitempty"`
	TreeStateSHA    string   `json:"tree_state_sha,omitempty"`
	WorktreeTreeSHA string   `json:"worktree_tree_sha,omitempty"`
	WorktreeBaseSHA string   `json:"worktree_base_sha,omitempty"`
	EntrySeq        int      `json:"entry_seq"`
	PrevHash        string   `json:"prev_hash"`
	WorkerCount     int      `json:"worker_count,omitempty"`
	Workers         []string `json:"workers,omitempty"`
	Action          string   `json:"action,omitempty"`
	TaskID          string   `json:"task_id,omitempty"`
	Message         string   `json:"message,omitempty"`
	Source          string   `json:"source,omitempty"`
	RunID           string   `json:"run_id,omitempty"`
}

type ledgerEntryWire struct {
	TS              string          `json:"ts,omitempty"`
	Cycle           json.RawMessage `json:"cycle,omitempty"`
	CycleLabel      string          `json:"cycle_label,omitempty"`
	Role            string          `json:"role,omitempty"`
	Kind            string          `json:"kind,omitempty"`
	Model           string          `json:"model,omitempty"`
	ExitCode        int             `json:"exit_code,omitempty"`
	DurationS       string          `json:"duration_s,omitempty"`
	ArtifactPath    string          `json:"artifact_path,omitempty"`
	ArtifactSHA256  string          `json:"artifact_sha256,omitempty"`
	ChallengeToken  string          `json:"challenge_token,omitempty"`
	GitHEAD         string          `json:"git_head,omitempty"`
	TreeStateSHA    string          `json:"tree_state_sha,omitempty"`
	WorktreeTreeSHA string          `json:"worktree_tree_sha,omitempty"`
	WorktreeBaseSHA string          `json:"worktree_base_sha,omitempty"`
	EntrySeq        int             `json:"entry_seq,omitempty"`
	PrevHash        string          `json:"prev_hash,omitempty"`
	WorkerCount     int             `json:"worker_count,omitempty"`
	Workers         []string        `json:"workers,omitempty"`
	Action          string          `json:"action,omitempty"`
	TaskID          string          `json:"task_id,omitempty"`
	Message         string          `json:"message,omitempty"`
	Source          string          `json:"source,omitempty"`
	RunID           string          `json:"run_id,omitempty"`
}

func (e *LedgerEntry) UnmarshalJSON(data []byte) error {
	var wire ledgerEntryWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	e.TS = wire.TS
	e.CycleLabel = wire.CycleLabel
	e.Role = wire.Role
	e.Kind = wire.Kind
	e.Model = wire.Model
	e.ExitCode = wire.ExitCode
	e.DurationS = wire.DurationS
	e.ArtifactPath = wire.ArtifactPath
	e.ArtifactSHA256 = wire.ArtifactSHA256
	e.ChallengeToken = wire.ChallengeToken
	e.GitHEAD = wire.GitHEAD
	e.TreeStateSHA = wire.TreeStateSHA
	e.WorktreeTreeSHA = wire.WorktreeTreeSHA
	e.WorktreeBaseSHA = wire.WorktreeBaseSHA
	e.EntrySeq = wire.EntrySeq
	e.PrevHash = wire.PrevHash
	e.WorkerCount = wire.WorkerCount
	e.Workers = wire.Workers
	e.Action = wire.Action
	e.TaskID = wire.TaskID
	e.Message = wire.Message
	e.Source = wire.Source
	e.RunID = wire.RunID
	return e.setCycle(wire.Cycle)
}

func (e *LedgerEntry) setCycle(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case 'n':
		if !bytes.Equal(trimmed, []byte("null")) {
			return fmt.Errorf("ledger cycle: unsupported JSON value %q", trimmed)
		}
		return nil
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return fmt.Errorf("ledger cycle: %w", err)
		}
		e.CycleLabel = s
		e.Cycle = 0
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		var n float64
		if err := json.Unmarshal(trimmed, &n); err != nil {
			return fmt.Errorf("ledger cycle: %w", err)
		}
		if n != float64(int64(n)) {
			return fmt.Errorf("ledger cycle: fractional value %v not allowed", n)
		}
		if n < math.MinInt32 || n > math.MaxInt32 {
			return fmt.Errorf("ledger cycle: value %v out of range", n)
		}
		e.Cycle = int(n)
	default:
		return fmt.Errorf("ledger cycle: unsupported JSON value %q", trimmed)
	}
	return nil
}

type CompletionContract string

const (
	CompletionArtifact         CompletionContract = "artifact"
	CompletionStdout           CompletionContract = "stdout"
	CompletionGit              CompletionContract = "git"
	CompletionWorktreeEvidence CompletionContract = "worktree-evidence"
)

type BridgeRequest struct {
	CLI                 string             `json:"cli"`
	Profile             string             `json:"profile"`
	Model               string             `json:"model"`
	Prompt              string             `json:"prompt"`
	Workspace           string             `json:"workspace"`
	Worktree            string             `json:"worktree,omitempty"`
	RunID               string             `json:"run_id,omitempty"`
	ProjectRoot         string             `json:"project_root,omitempty"`
	StdoutLog           string             `json:"stdout_log,omitempty"`
	StderrLog           string             `json:"stderr_log,omitempty"`
	ArtifactPath        string             `json:"artifact_path,omitempty"`
	SecondaryArtifacts  []string           `json:"secondary_artifacts,omitempty"`
	Completion          CompletionContract `json:"completion,omitempty"`
	Agent               string             `json:"agent,omitempty"`
	Contract            string             `json:"contract,omitempty"`
	Cycle               int                `json:"cycle,omitempty"`
	Attempt             int                `json:"attempt,omitempty"`
	ChainAttempt        bool               `json:"chain_attempt,omitempty"`
	FreshSession        bool               `json:"fresh_session,omitempty"`
	BudgetScale         float64            `json:"budget_scale,omitempty"`
	RequireSandbox      bool               `json:"require_sandbox,omitempty"`
	Env                 map[string]string  `json:"env,omitempty"`
	ExtraFlags          []string           `json:"extra_flags,omitempty"`
	PermissionMode      string             `json:"permission_mode,omitempty"`
	InteractivePolicy   string             `json:"interactive_policy,omitempty"`
	SystemPrompt        string             `json:"system_prompt,omitempty"`
	Skills              []string           `json:"skills,omitempty"`
	CorrectionDirective string             `json:"correction_directive,omitempty"`
	OperatorDirectives  string             `json:"operator_directives,omitempty"`
	SessionName         string             `json:"session_name,omitempty"`
}

type BridgeResponse struct {
	ExitCode           int        `json:"exit_code"`
	Stdout             string     `json:"stdout"`
	Stderr             string     `json:"stderr"`
	CostUSD            float64    `json:"cost_usd"`
	Tokens             TokenUsage `json:"tokens"`
	DurationMS         int64      `json:"duration_ms"`
	BootMS             int64      `json:"boot_ms,omitempty"`
	UsageExhausted     bool       `json:"usage_exhausted,omitempty"`
	CauseCode          string     `json:"cause_code,omitempty"`
	DispatchID         string     `json:"dispatch_id,omitempty"`
	PaneCapture        string     `json:"pane_capture,omitempty"`
	SessionRecoverable bool       `json:"session_recoverable,omitempty"`
}

type BridgeProbe struct {
	Version string            `json:"version"`
	CLIs    map[string]string `json:"clis"`
}

type GuardInput struct {
	ToolName       string
	ToolInput      map[string]any
	CWD            string
	CycleStatePath string
}

type GuardDecision struct {
	Allow  bool
	Reason string
	Alarm  bool
}
