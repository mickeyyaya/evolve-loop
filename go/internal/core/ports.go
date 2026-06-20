package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
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

// (Retired in Workstream B.) The historical core.Sandbox port + SandboxProfile
// struct were placeholders whose signature never matched the actual
// adapters/sandbox.Sandbox.Exec impl, and nothing wired them. CLI-agnostic
// confinement now lives at the bridge layer: bridge.Deps.SandboxWrap calls
// adapters/sandbox.GenerateSBPL / BwrapPrefix directly. Removed to avoid the
// dead port collecting future implementations.

// Guard runs a trust-kernel guard. Impls live in internal/guards.
type Guard interface {
	Name() string
	Decide(ctx context.Context, in GuardInput) GuardDecision
}

// (Legacy speculative Observer interface removed in cycle-122 Fix 3 /
// ADR-0030 — it was scaffolding with zero callers. The live interface
// is in observer.go with the Start(ctx, phase, req)→cancel shape that
// the orchestrator actually wires from RunCycle.)

// State mirrors the .evolve/state.json schema (subset used by orchestrator).
// Full field set is round-tripped through encoding/json by the storage
// adapter; this struct exposes only the orchestrator-load-bearing fields.
type State struct {
	LastUpdated     string          `json:"lastUpdated"`
	LastCycleNumber int             `json:"lastCycleNumber"`
	Version         int             `json:"version"`
	CurrentBatch    BatchAccrual    `json:"currentBatch"`
	FailedAt        []FailedRecord  `json:"failedApproaches,omitempty"`
	CarryoverTodos  []CarryoverTodo `json:"carryoverTodos,omitempty"`
	// SetupCompletedAt / SetupVersion are the first-run onboarding marker
	// (RFC3339 stamp + version) written by `evolve setup complete`. Empty
	// SetupCompletedAt means setup has never run → `evolve loop` prints a
	// one-line non-blocking nudge. omitempty keeps pre-setup state.json
	// byte-clean. NOTE: this struct is a SUBSET view of state.json (the
	// orchestrator's WriteState would drop unmodeled keys), so the setup
	// marker is written via a lossless raw-merge — never via WriteState.
	SetupCompletedAt string `json:"setupCompletedAt,omitempty"`
	SetupVersion     int    `json:"setupVersion,omitempty"`
	// TriageThroughput is the rolling window (last 5 floor-bearing PASS
	// cycles) of coverage floors passed per cycle — the observed builder
	// throughput that bounds triage's per-cycle floor commitments (R9,
	// inbox coverage-floor-overpacking). Ops live in internal/triagecap.
	TriageThroughput []TriageThroughputEntry `json:"triageThroughput,omitempty"`
	// StateRevision counts writes made through Storage.UpdateState (CA.3
	// OCC): ++ per serialized RMW. 0 (omitted) = never touched by
	// UpdateState; a gap/repeat in the sequence betrays a writer that
	// bypassed the lock. Additive field — single-mode byte-stable.
	StateRevision int `json:"stateRevision,omitempty"`
	// LastAllocatedCycleNumber is the CA.4 allocation lease: the highest
	// cycle number ever minted (≠ LastCycleNumber, the highest COMPLETED).
	// Bumped atomically via UpdateState before a run starts; a crashed run
	// burns its number. Additive field — single-mode byte-stable.
	LastAllocatedCycleNumber int `json:"lastAllocatedCycleNumber,omitempty"`
}

// TriageThroughputEntry is one observed cycle in the triage-capacity rolling
// window: a PASS cycle and how many coverage floors it committed and passed.
type TriageThroughputEntry struct {
	Cycle  int `json:"cycle"`
	Floors int `json:"floors"`
}

// BatchAccrual tracks per-dispatcher-invocation cost.
type BatchAccrual struct {
	CycleAccruedCostUSD float64 `json:"cycleAccruedCostUSD"`
	GoalHash            string  `json:"goalHash,omitempty"`
}

// FailedRecord captures a non-PASS cycle outcome — what the bash side
// writes into state.json:failedApproaches[]. JSON tags use camelCase to
// match the on-disk shape (see legacy/scripts/dispatch/subagent-run.sh +
// failure-classifications.sh). The Classification + ExpiresAt fields are
// consumed by failureadapter to decide RETRY/BLOCK/PROCEED.
type FailedRecord struct {
	TS                string   `json:"ts,omitempty"`
	Cycle             int      `json:"cycle"`
	Verdict           string   `json:"verdict"`
	Classification    string   `json:"classification,omitempty"`
	RecordedAt        string   `json:"recordedAt,omitempty"`
	ExpiresAt         string   `json:"expiresAt,omitempty"`
	AuditReportPath   string   `json:"auditReportPath,omitempty"`
	AuditReportSHA256 string   `json:"auditReportSha256,omitempty"`
	GitHead           string   `json:"gitHead,omitempty"`
	TreeStateSHA      string   `json:"treeStateSha,omitempty"`
	Defects           []string `json:"defects,omitempty"`
	Retrospected      bool     `json:"retrospected"`
	Summary           string   `json:"summary,omitempty"`
}

// CarryoverTodo is operator-queued work surfaced cycle-to-cycle.
type CarryoverTodo struct {
	ID             string `json:"id"`
	Action         string `json:"action"`
	Priority       string `json:"priority"`
	FirstSeenCycle int    `json:"first_seen_cycle"`
	CyclesUnpicked int    `json:"cycles_unpicked"`
}

// CycleState mirrors .evolve/cycle-state.json (transient per-cycle).
type CycleState struct {
	CycleID         int      `json:"cycle_id"`
	Phase           string   `json:"phase"`
	StartedAt       string   `json:"started_at"`
	PhaseStartedAt  string   `json:"phase_started_at"`
	ActiveAgent     string   `json:"active_agent,omitempty"`
	ActiveWorktree  string   `json:"active_worktree,omitempty"`
	CompletedPhases []string `json:"completed_phases,omitempty"`
	WorkspacePath   string   `json:"workspace_path"`
	IntentRequired  bool     `json:"intent_required"`
	// RunID is the CA.5 event-sourced run identity: the ULID RunCycle mints
	// for this run, also stamped on every ledger entry the run emits.
	// Additive omitempty field — pre-CA.5 cycle-state files decode/encode
	// unchanged.
	RunID string `json:"run_id,omitempty"`
	// WorktreeBaseSHA is the per-cycle worktree HEAD at creation == the cycle
	// base. Persisted so the crash-resume path (RunCycleFromPhase) can run the
	// cycle-156 build-commit normalize, which RunCycle previously drove from a
	// run-local variable. Empty (omitted) for pre-field checkpoints and
	// worktree-less cycles → the normalize degrades to a no-op.
	WorktreeBaseSHA string `json:"worktree_base_sha,omitempty"`
}

// LedgerEntry is one .jsonl line in .evolve/ledger.jsonl.
//
// The cycle field has a custom unmarshaler that accepts int (canonical)
// or string (legacy manual entries, e.g. "manual-release-v10.16.0").
// On-disk bytes are never rewritten — doing so would cascade SHA256
// hash-chain breaks through every subsequent entry.
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
	// WorktreeTreeSHA is the git tree SHA of the per-cycle worktree's WORKING
	// state (all changes staged) at audit time — the tree ship will commit.
	// Written by the orchestrator's audit-binding entry so ship's pre/post-merge
	// tree-drift check binds to the audited CHANGES, not the auditor's
	// HEAD^{tree} (which is the unchanged base in the worktree flow, cycle-152).
	WorktreeTreeSHA string   `json:"worktree_tree_sha,omitempty"`
	EntrySeq        int      `json:"entry_seq"`
	PrevHash        string   `json:"prev_hash"`
	WorkerCount     int      `json:"worker_count,omitempty"`
	Workers         []string `json:"workers,omitempty"`
	// Action carries the decision verb for self-heal events (e.g. "extend" or
	// "pause" for stop_review entries). Empty for all other entry kinds.
	Action string `json:"action,omitempty"`
	// Message carries a human-readable detail string for self-heal events
	// (e.g. the stop-reviewer's justification text). Empty for other kinds.
	Message string `json:"message,omitempty"`
	// Source identifies the skip-decision origin for phase_skipped entries.
	// Values: router | psmas | content. Omitted for all other entry kinds.
	Source string `json:"source,omitempty"`
	// RunID is the event-sourced run identity (CA.2, Track C-A): the ULID
	// minted per cycle run, threaded into every entry that run emits (CA.5)
	// so concurrent runs' entries are attributable. Empty (omitted) for
	// single-mode and all pre-CA.2 lines — additive field only, byte-stable.
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
	EntrySeq        int             `json:"entry_seq,omitempty"`
	PrevHash        string          `json:"prev_hash,omitempty"`
	WorkerCount     int             `json:"worker_count,omitempty"`
	Workers         []string        `json:"workers,omitempty"`
	Action          string          `json:"action,omitempty"`
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
	e.EntrySeq = wire.EntrySeq
	e.PrevHash = wire.PrevHash
	e.WorkerCount = wire.WorkerCount
	e.Workers = wire.Workers
	e.Action = wire.Action
	e.Message = wire.Message
	e.Source = wire.Source
	e.RunID = wire.RunID

	// Route the cycle field: int → Cycle, string → CycleLabel.
	if len(wire.Cycle) == 0 {
		return nil
	}
	trimmed := bytes.TrimSpace(wire.Cycle)
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return fmt.Errorf("ledger cycle: %w", err)
		}
		e.CycleLabel = s
		e.Cycle = 0
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Numeric — accept whole-number floats too. Range-check explicitly
		// against int32 bounds (instead of MaxInt) so behaviour is
		// identical on 32-bit and 64-bit targets — cycle numbers > 2^31
		// would never realistically appear, but a silent truncation on a
		// 32-bit builder would be a surprise.
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
	// RunID is the CA.5 run identity (CB.5): the bridge namespaces tmux
	// session names with r<runid8> and stamps the per-run session registry.
	RunID string `json:"run_id,omitempty"`
	// ProjectRoot is the absolute path to the main repo root. Needed by the
	// bridge's SandboxWrap (Workstream B) to set RepoRoot read-only while
	// allowing writes to Worktree+Workspace. Optional for back-compat: a zero
	// value disables sandbox confinement for that call (degraded — the trust
	// kernel's pre-B Claude-only PreToolUse hooks remain in effect).
	ProjectRoot  string `json:"project_root,omitempty"`
	StdoutLog    string `json:"stdout_log,omitempty"`
	StderrLog    string `json:"stderr_log,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"` // adapter requires non-empty
	// Completion selects the phase-completion contract (ADR-0027): "" /
	// "artifact" = poll the artifact file (default); "stdout" = complete on
	// REPL-idle for agents that print their answer and write no file (the
	// router/advisor). Only the *-tmux drivers honor it; others ignore it.
	Completion string            `json:"completion,omitempty"`
	Agent      string            `json:"agent,omitempty"` // role label
	Cycle      int               `json:"cycle,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	ExtraFlags []string          `json:"extra_flags,omitempty"` // direct inner-CLI pass-through (after `--`)
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
	// it). The swarm harness (ADR-0032) sets this and REGISTERS the name before
	// calling Launch, so a worker cancelled mid-spawn can still be reaped by name
	// (closing the orphan-on-cancel gap). A named session is preserved by the
	// driver's own cleanup — the caller owns teardown.
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
	// BootMS is the cold-boot latency the tmux-REPL driver spent from
	// tmux new-session to the REPL prompt marker appearing — pure dispatch
	// overhead, paid before the prompt is delivered (ADR-0043 A0). 0 when no
	// cold boot happened (a resumed/warm named session, or a headless driver).
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
type GuardDecision struct {
	Allow  bool
	Reason string
}
