package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Phase is the typed identity of an orchestrator lifecycle stage.
// Stringly-backed for JSON portability.
type Phase string

const (
	PhaseStart        Phase = "start"
	PhaseIntent       Phase = "intent"
	PhaseScout        Phase = "scout"
	PhaseTriage       Phase = "triage"
	PhaseTDD          Phase = "tdd"
	PhaseBuildPlanner Phase = "build-planner"
	PhaseSwarmPlan    Phase = "swarm-plan"
	PhaseBuild        Phase = "build"
	PhaseAudit        Phase = "audit"
	PhaseShip         Phase = "ship"
	PhaseRetro        Phase = "retro"
	// PhaseDebugger is the recovery phase the advisor can recommend when a
	// phase (typically ship) returns a structured error/blocker. It receives
	// the ShipError on its input, diagnoses the root cause, and emits a
	// debug-decision (RESHIP / RERUN_PHASE / BLOCK) the orchestrator executes.
	// OPTIONAL — never on the mandatory spine.
	PhaseDebugger Phase = "debugger"
	PhaseEnd      Phase = "end"
)

// String implements fmt.Stringer.
func (p Phase) String() string { return string(p) }

// IsValid reports whether p is one of the known phase constants.
func (p Phase) IsValid() bool {
	switch p {
	case PhaseStart, PhaseIntent, PhaseScout, PhaseTriage,
		PhaseTDD, PhaseBuildPlanner, PhaseSwarmPlan, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseDebugger, PhaseEnd:
		return true
	}
	return false
}

// Verdict constants — the four outcomes a phase may emit. These match
// the EGPS gate vocabulary (CLAUDE.md env-var table: WARN removed at
// v10.0.0 but still accepted by Audit for the pre-EGPS soft-start
// boundary; SKIPPED used when a phase opted out, e.g. EVOLVE_TRIAGE_DISABLE).
const (
	VerdictPASS    = "PASS"
	VerdictFAIL    = "FAIL"
	VerdictWARN    = "WARN"
	VerdictSKIPPED = "SKIPPED"
)

// CycleOutcome constants — cycle-level FinalVerdict labels emitted by
// finalizeOutcome. Distinct from the per-phase Verdict* set because a
// cycle outcome covers multiple phases plus commit-movement signal.
// They disambiguate the bare "SKIPPED" verdict that previously conflated
// an inline build-ship, a fluent-mode advisory, and a no-signal noop.
const (
	CycleOutcomeShippedViaBuild      = "SHIPPED_VIA_BUILD"
	CycleOutcomeSkippedAuditAdvisory = "SKIPPED_AUDIT_ADVISORY"
	CycleOutcomeSkippedUnknown       = "SKIPPED_UNKNOWN"
)

// IsVerdict reports whether s is one of the canonical verdict strings.
// Case- and whitespace-sensitive — guards against silent typos.
func IsVerdict(s string) bool {
	switch s {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}

// BudgetEnvelope is the per-call budget envelope passed to each phase.
type BudgetEnvelope struct {
	MaxUSD      float64 `json:"max_usd"`
	BatchCapUSD float64 `json:"batch_cap_usd"`
}

// TokenUsage records the LLM token counts attributed to a phase run.
type TokenUsage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read"`
	CacheWrite int `json:"cache_write"`
}

// Diagnostic is a single warning/error attached to a PhaseResponse.
type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// PhaseRequest is the input envelope to PhaseRunner.Run. JSON-tagged
// so the subprocess override path (pkg/phaseproto) can serialise the
// same struct over stdin/stdout.
type PhaseRequest struct {
	Cycle int `json:"cycle"`
	// ProjectRoot is the MAIN repo root — the RUNTIME-DATA root. All `.evolve/`
	// state (runs/, evals/, the ledger, baselines) lives here, and it is what a
	// subprocess sees as EVOLVE_PROJECT_ROOT. Stable across the whole cycle.
	ProjectRoot string `json:"project_root"`
	// Workspace is this cycle's per-phase scratch dir, under
	// <ProjectRoot>/.evolve/runs/cycle-<N>/ — where artifacts, phase logs, and
	// the observer's sinks are written.
	Workspace string `json:"workspace"`
	// Worktree is the per-cycle git worktree — the SHIPPED-TREE root. A
	// source-writing phase edits here, and an EGPS predicate's `go test`
	// compiles here (acssuite runs predicates with cwd=Worktree while `.evolve/`
	// still resolves to ProjectRoot via EVOLVE_PROJECT_ROOT — the intentional
	// dual root, issue #9 + #12). Empty for read-only phases, which then run
	// against ProjectRoot. Keep ProjectRoot (data) and Worktree (code) distinct:
	// collapsing them reintroduces the cycle-190 "predicate ran against main" bug.
	Worktree      string            `json:"worktree"`
	GoalHash      string            `json:"goal_hash"`
	Context       map[string]string `json:"context,omitempty"`
	Budget        BudgetEnvelope    `json:"budget"`
	PreviousPhase string            `json:"previous_phase,omitempty"`
	Env           map[string]string `json:"env,omitempty"`

	// CorrectionDirective is set by the orchestrator's contract-correction loop
	// on a re-dispatch after a deliverable reject; the runner copies it into the
	// BridgeRequest. Empty on the first dispatch.
	CorrectionDirective string `json:"correction_directive,omitempty"`

	// Spec is the brick's own declarative definition. A spec-driven (kind:llm)
	// phase reads all of its behavior from here; built-in Go phases ignore it.
	// Additive in Stage 1 (nothing sets it until the orchestrator becomes a pure
	// driver in Stage 3).
	Spec *phasespec.PhaseSpec `json:"spec,omitempty"`

	// UpstreamSignals is the accumulating signal bus piped in from prior phases,
	// namespaced <phase>.<key> (e.g. "build.files_touched"). A phase keys its
	// own routing/classify decisions off these without re-reading artifacts.
	UpstreamSignals map[string]any `json:"upstream_signals,omitempty"`
}

// PhaseResponse is the output envelope from PhaseRunner.Run.
type PhaseResponse struct {
	Phase        string       `json:"phase"`
	Verdict      string       `json:"verdict"`
	ArtifactsDir string       `json:"artifacts_dir"`
	NextPhase    string       `json:"next_phase,omitempty"`
	CostUSD      float64      `json:"cost_usd"`
	Tokens       TokenUsage   `json:"tokens"`
	DurationMS   int64        `json:"duration_ms"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`

	// Signals is the namespaced signal map this phase emits onto the bus — the
	// typed "message on the pipe." The router (Stage 3) consumes these directly
	// instead of re-parsing markdown. Keys are <phase>.<key>. Additive in
	// Stage 1: emitted but not yet read by routing.
	Signals map[string]any `json:"signals,omitempty"`

	// CommitSHA is the commit that anchors this phase's deliverable
	// (ADR-0027 commit-as-evidence). Empty when the phase committed nothing.
	CommitSHA string `json:"commit_sha,omitempty"`
}

// PhaseRunner runs a single phase. The orchestrator never knows which
// runner impl is in play (in-process vs subprocess) — that's the
// independence guarantee for Approach C.
type PhaseRunner interface {
	Name() string
	Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error)
}
