package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
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

// Guard runs a trust-kernel guard. Impls live in internal/guards.
type Guard interface {
	Name() string
	Decide(ctx context.Context, in GuardInput) GuardDecision
}

// State mirrors the .evolve/state.json schema (subset used by orchestrator);
// these are aliases of the pure on-disk DTOs defined in internal/cyclestate.
type (
	State                 = cyclestate.State
	CycleState            = cyclestate.CycleState
	BatchAccrual          = cyclestate.BatchAccrual
	FailedRecord          = cyclestate.FailedRecord
	CarryoverTodo         = cyclestate.CarryoverTodo
	TriageThroughputEntry = cyclestate.TriageThroughputEntry
)

// LedgerEntry is one .jsonl line in .evolve/ledger.jsonl. The cycle field has
// a custom unmarshaler accepting int (canonical) or string (legacy manual
// entries, e.g. "manual-release-v10.16.0"). On-disk bytes are never
// rewritten — doing so would cascade SHA256 hash-chain breaks through every
// subsequent entry.
type LedgerEntry struct {
	TS             string `json:"ts"`
	Cycle          int    `json:"cycle"`
	CycleLabel     string `json:"cycle_label,omitempty"`
	Role           string `json:"role"`
	Kind           string `json:"kind"`
	Model          string `json:"model,omitempty"`
	ExitCode       int    `json:"exit_code"`
	DurationS      string `json:"duration_s,omitempty"`
	ArtifactPath   string `json:"artifact_path,omitempty"`
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
	ChallengeToken string `json:"challenge_token,omitempty"`
	GitHEAD        string `json:"git_head,omitempty"`
	TreeStateSHA   string `json:"tree_state_sha,omitempty"`
	// WorktreeTreeSHA is the git tree SHA of the per-cycle worktree's working
	// state (all changes staged) at audit time — the tree ship will commit. It
	// binds ship's tree-drift check to the audited CHANGES, not the auditor's
	// unchanged HEAD^{tree} base.
	WorktreeTreeSHA string   `json:"worktree_tree_sha,omitempty"`
	WorktreeBaseSHA string   `json:"worktree_base_sha,omitempty"`
	EntrySeq        int      `json:"entry_seq"`
	PrevHash        string   `json:"prev_hash"`
	WorkerCount     int      `json:"worker_count,omitempty"`
	Workers         []string `json:"workers,omitempty"`
	// Action carries the decision verb for self-heal events (e.g. "extend" or
	// "pause" for stop_review entries) and for inbox-lifecycle entries (e.g.
	// "claim", "promote"). Empty for all other entry kinds.
	Action string `json:"action,omitempty"`
	// TaskID names the inbox item an inbox-lifecycle entry moved. Empty for
	// all other entry kinds.
	TaskID string `json:"task_id,omitempty"`
	// Message carries a human-readable detail string for self-heal events
	// (e.g. the stop-reviewer's justification text). Empty for other kinds.
	Message string `json:"message,omitempty"`
	// Source identifies the skip-decision origin for phase_skipped entries.
	// Values: router | psmas | content. Omitted for all other entry kinds.
	Source string `json:"source,omitempty"`
	// RunID is the event-sourced run identity: the ULID minted per cycle run,
	// threaded into every entry that run emits so concurrent runs' entries are
	// attributable. Empty for single-mode and pre-existing lines (additive,
	// byte-stable).
	RunID string `json:"run_id,omitempty"`
}

// ledgerEntryWire is the JSON-facing twin of LedgerEntry. Cycle is a
// json.RawMessage so the custom unmarshaler can route int vs string
// without recursing back into LedgerEntry.UnmarshalJSON.
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

// UnmarshalJSON accepts cycle as int, whole-number float, or string.
// String form goes to CycleLabel; fractional floats, objects, and arrays error out.
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

// BridgeRequest is the input to Bridge.Launch. Field shape mirrors the
// flag surface of `tools/agent-bridge/bin/bridge launch`. The adapter
// writes Prompt to a file under Workspace before invoking the bridge
// subprocess (callers don't manage tmp-file lifecycle).
type BridgeRequest struct {
	CLI       string `json:"cli"`       // claude-p | claude-tmux | codex | agy
	Profile   string `json:"profile"`   // absolute path to .evolve/profiles/<name>.json
	Model     string `json:"model"`     // haiku | sonnet | opus | auto | gpt-* | gemini-*
	Prompt    string `json:"prompt"`    // prompt body; adapter materializes as a file
	Workspace string `json:"workspace"` // absolute path; bridge writes outputs here
	Worktree  string `json:"worktree,omitempty"`
	// RunID namespaces the bridge's tmux session names with r<runid8> and
	// stamps the per-run session registry.
	RunID string `json:"run_id,omitempty"`
	// ProjectRoot is the absolute path to the main repo root, used by the
	// bridge's SandboxWrap to set RepoRoot read-only while allowing writes to
	// Worktree+Workspace. A zero value disables sandbox confinement for that
	// call (degraded: the pre-sandbox Claude-only PreToolUse hooks remain in
	// effect).
	ProjectRoot  string `json:"project_root,omitempty"`
	StdoutLog    string `json:"stdout_log,omitempty"`
	StderrLog    string `json:"stderr_log,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"` // adapter requires non-empty
	// SecondaryArtifacts are additional deliverables the phase contract
	// requires beyond ArtifactPath (absolute paths). The completion detector
	// holds phase-complete until every one exists (existence only; the settle
	// window stays primary-only); the artifact-timeout final poll still
	// completes without them, and the phase gate then reports the absence
	// loudly.
	SecondaryArtifacts []string           `json:"secondary_artifacts,omitempty"`
	Completion         CompletionContract `json:"completion,omitempty"`
	Agent              string             `json:"agent,omitempty"` // role label
	// Contract selects the deliverable protocol independently from Agent. Empty
	// defaults to Agent for backward compatibility. PhaseAdvisor uses this when
	// one router persona produces plan, replan, and proposal artifacts.
	Contract string `json:"contract,omitempty"`
	Cycle    int    `json:"cycle,omitempty"`
	// Attempt is the 1-based fallback-retry ordinal for this Launch: the
	// caller's fallback loop calls Launch once per CLI candidate, and Attempt
	// lets each call's llm-calls.ndjson record be distinguished so the
	// double-dispatch waste class is measurable. Zero (unset) is treated as
	// attempt 1.
	Attempt int `json:"attempt,omitempty"`
	// ChainAttempt marks one attempt of a chain walk (llmroute.DispatchTiered in
	// the runner, the advisor's Dispatch, bridgechain.Walking itself). A
	// chain-walking bridge handle passes such a request straight through instead
	// of resolving and walking the chain again; a launch WITHOUT it is a caller
	// that never heard of the chain and gets the walk by construction.
	ChainAttempt bool `json:"chain_attempt,omitempty"`
	// BudgetScale scales the launch's artifact-wait budget (a large cycle's
	// build gets a longer deadline). 0 or 1 = unscaled; <1 never shrinks. The
	// engine applies it to the per-agent policy base, or the builtin when the
	// map has no entry.
	BudgetScale float64 `json:"budget_scale,omitempty"`
	// RequireSandbox fails the launch closed when filesystem confinement is
	// unavailable. Activated Build explanation contracts set this for Builder.
	RequireSandbox bool              `json:"require_sandbox,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	ExtraFlags     []string          `json:"extra_flags,omitempty"` // direct inner-CLI pass-through (after `--`)
	// PermissionMode is the resolved per-phase permission mode (the
	// EVOLVE_<AGENT>_PERMISSION_MODE override the runner resolves with the
	// agent name). The bridge realizes it per-CLI via the LaunchIntent —
	// passed as typed config, NOT a raw flag, so it never leaks into a
	// non-claude launch command. Empty = profile/realizer default (bypass).
	PermissionMode string `json:"permission_mode,omitempty"`
	// InteractivePolicy is the resolved per-phase prompt interaction policy.
	// The runner resolves explicit per-agent request env, then profile config,
	// and passes the result as typed config. Empty = adapter default.
	InteractivePolicy string `json:"interactive_policy,omitempty"`
	// SystemPrompt is the per-agent launch-time rules block prepended to the
	// prompt body (facet B). Resolved by the runner via systemprompt.Resolve.
	SystemPrompt string `json:"system_prompt,omitempty"`
	// Skills is the ordered list of policy-resolved skill-overlay names to
	// preload for this dispatch. The bridge adapter materializes each named
	// skill's SKILL.md persona body into a prompt prefix just above the
	// profile Rules block. Empty means no overlay.
	Skills []string `json:"skills,omitempty"`
	// CorrectionDirective, when non-empty, is prepended as a "## Correction"
	// block (the orchestrator's contract-correction retry — the previous
	// deliverable was rejected; fix it). Empty = no-op. See injectCorrectionPrefix.
	CorrectionDirective string `json:"correction_directive,omitempty"`
	// OperatorDirectives, when non-empty, is the rendered runtime operator-directives
	// block (internal/directives) snapshotted at cycle start. Prepended as a
	// "## Operator Directives" block so every phase agent sees the current global +
	// per-loop guidance. Empty = no-op (byte-identical). See injectOperatorDirectives.
	OperatorDirectives string `json:"operator_directives,omitempty"`
	// SessionName, when non-empty, pins the tmux session to a deterministic,
	// caller-controlled name (claude-tmux/*-tmux only; headless drivers ignore
	// it). The swarm harness sets this and registers the name before calling
	// Launch, so a worker cancelled mid-spawn can still be reaped by name. A
	// named session is preserved by the driver's own cleanup — the caller owns
	// teardown.
	SessionName string `json:"session_name,omitempty"`
}

// BridgeResponse is the bridge's JSON-parsed reply.
type BridgeResponse struct {
	ExitCode   int        `json:"exit_code"`
	Stdout     string     `json:"stdout"`
	Stderr     string     `json:"stderr"`
	CostUSD    float64    `json:"cost_usd"`
	Tokens     TokenUsage `json:"tokens"`
	DurationMS int64      `json:"duration_ms"`
	// BootMS is the cold-boot latency the tmux-REPL driver spent from tmux
	// new-session to the REPL prompt marker appearing — pure dispatch overhead
	// paid before the prompt is delivered. 0 when no cold boot happened (a
	// resumed/warm named session, or a headless driver).
	BootMS int64 `json:"boot_ms,omitempty"`
}

// BridgeProbe is what bridge reports about its environment + CLIs.
type BridgeProbe struct {
	Version string            `json:"version"`
	CLIs    map[string]string `json:"clis"` // cli name → tier (full/degraded/none)
}

// GuardInput is the typed input to a guard's Decide() method.
type GuardInput struct {
	ToolName       string         // "Bash" | "Edit" | "Write" | "Agent" | "WebSearch" | …
	ToolInput      map[string]any // raw stdin JSON tool_input
	CWD            string
	CycleStatePath string // optional; defaults to <CWD>/.evolve/cycle-state.json
}

// GuardDecision is what a guard returns. Allow=true → exit 0; Allow=false → exit 2.
// Reason is logged to .evolve/guards.log and written to stderr.
//
// Alarm marks a deny as an INTEGRITY VIOLATION (not a routine boundary deny) — a
// phase agent attempted to modify the pipeline control plane that grades it. When
// set, the guard runner additionally emits a CRITICAL record to
// .evolve/integrity-alarm.jsonl so the violation is loud and auditable, not just
// a silent exit-2.
type GuardDecision struct {
	Allow  bool
	Reason string
	Alarm  bool
}
