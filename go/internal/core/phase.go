package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Phase, the phase/verdict/cycle-outcome vocabulary, and IsVerdict are defined
// in the zero-dependency leaf internal/cyclestate (Stable-Dependencies: the
// most-depended-on symbols in the module now depend on nothing themselves).
// These re-exports — a type alias + const re-declarations — keep the ~1k
// existing core.Phase / core.Phase* / core.Verdict* / core.IsVerdict call
// sites unchanged; new code may depend on internal/cyclestate directly. The
// Phase methods (String, IsValid) come with the alias, so callers still write
// p.String() / p.IsValid().
type Phase = cyclestate.Phase

const (
	PhaseStart        = cyclestate.PhaseStart
	PhaseIntent       = cyclestate.PhaseIntent
	PhaseScout        = cyclestate.PhaseScout
	PhaseTriage       = cyclestate.PhaseTriage
	PhaseTDD          = cyclestate.PhaseTDD
	PhaseBuildPlanner = cyclestate.PhaseBuildPlanner
	PhaseSwarmPlan    = cyclestate.PhaseSwarmPlan
	PhaseBuild        = cyclestate.PhaseBuild
	PhaseAudit        = cyclestate.PhaseAudit
	PhaseShip         = cyclestate.PhaseShip
	PhaseRetro        = cyclestate.PhaseRetro
	PhaseDebugger     = cyclestate.PhaseDebugger
	PhaseEnd          = cyclestate.PhaseEnd
)

const (
	VerdictPASS    = cyclestate.VerdictPASS
	VerdictFAIL    = cyclestate.VerdictFAIL
	VerdictWARN    = cyclestate.VerdictWARN
	VerdictSKIPPED = cyclestate.VerdictSKIPPED
)

const (
	CycleOutcomeShippedViaBuild      = cyclestate.CycleOutcomeShippedViaBuild
	CycleOutcomeSkippedAuditAdvisory = cyclestate.CycleOutcomeSkippedAuditAdvisory
	CycleOutcomeSkippedUnknown       = cyclestate.CycleOutcomeSkippedUnknown
)

// IsVerdict re-exports cyclestate.IsVerdict via a thin wrapper (not a var) so
// the symbol stays an immutable func — identical call signature, can't be
// reassigned by an importer, and renders as a function in godoc.
func IsVerdict(s string) bool { return cyclestate.IsVerdict(s) }

// TokenUsage, Diagnostic, and CycleResult — the cycle/phase execution-result
// value types — are defined in the internal/cyclestate leaf and re-exported here
// so existing call sites are unchanged (see cyclestate/result.go).
type (
	TokenUsage          = cyclestate.TokenUsage
	Diagnostic          = cyclestate.Diagnostic
	CycleResult         = cyclestate.CycleResult
	SkippedPhase        = cyclestate.SkippedPhase
	SystemFailureSignal = cyclestate.SystemFailureSignal
)

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
	// dual root, issue #9 + #12). Since CB.1 it is set for EVERY phase (cwd
	// isolation is universal; write permission stays on the worktreePhase
	// axis); empty only when provisioning failed — the degraded mode where
	// phases run against ProjectRoot. Keep ProjectRoot (data) and Worktree
	// (code) distinct: collapsing them reintroduces the cycle-190 "predicate
	// ran against main" bug.
	Worktree string `json:"worktree"`
	// RunID is the CA.5 event-sourced run identity, threaded to every phase
	// dispatch (CB.5) so the bridge mints run-scoped tmux session names
	// (evolve-bridge-r<runid8>-…) and the per-run session registry records
	// the right owner. Empty on legacy/degraded paths — names then keep the
	// pre-CB.5 format.
	RunID         string            `json:"run_id,omitempty"`
	GoalHash      string            `json:"goal_hash"`
	Context       map[string]string `json:"context,omitempty"`
	PreviousPhase string            `json:"previous_phase,omitempty"`
	Env           map[string]string `json:"env,omitempty"`

	// BuildPlan is the build phase's upstream build-plan.md body, served via the
	// typed envelope instead of an ad-hoc disk read inside the phase (ADR-0050
	// Phase 3.7). Populated once at the dispatch seam, and only at
	// EVOLVE_PHASE_IO>=advisory with the planner enabled; empty at off/shadow so
	// the build phase reads disk exactly as before (byte-identical dispatch).
	BuildPlan string `json:"build_plan,omitempty"`

	// Input is the unified typed phase-I/O envelope (ADR-0050 Phase 3.10). It is
	// assembled once at the dispatch seam and ONLY at EVOLVE_PHASE_IO>=enforce; the
	// zero value at off/shadow/advisory keeps dispatch byte-identical (no phase
	// consumes it until the enforce cutover migrates readers off the Context map).
	// Excluded from JSON: PhaseInput seals its channels behind unexported fields so
	// it is not wire-serializable — the subprocess override path keeps using
	// Context, and in-core phases read this envelope when composing prompts before
	// any subprocess launch.
	Input phaseio.PhaseInput `json:"-"`

	// BypassPolicy skips policy.json pin enforcement for this phase.
	// Set by the orchestrator from CycleRequest.BypassPolicy; operators may
	// trigger it via --bypass-policy at the cycle or compose call site.
	BypassPolicy bool `json:"bypass_policy,omitempty"`
	// ComposePhases signals that phases are being run via `evolve compose`
	// (ad-hoc composition bypassing the state machine). The kernel guard
	// downgrades from BLOCK to WARN when this field is true. Replaces the
	// retired EVOLVE_COMPOSE_PHASES env signal (cycle-10 flag-reduction).
	ComposePhases bool `json:"compose_phases,omitempty"`
	// CorrectionDirective is set by the orchestrator's contract-correction loop
	// on a re-dispatch after a deliverable reject; the runner copies it into the
	// BridgeRequest. Empty on the first dispatch.
	CorrectionDirective string `json:"correction_directive,omitempty"`
	// OperatorDirectives is the rendered runtime operator-directives block the
	// orchestrator snapshots once at cycle start (internal/directives) and passes
	// to every phase this cycle; the runner copies it into the BridgeRequest.
	// Empty = no directives configured (byte-identical dispatch).
	OperatorDirectives string `json:"operator_directives,omitempty"`

	// Spec is the brick's own declarative definition. A spec-driven (kind:llm)
	// phase reads all of its behavior from here; built-in Go phases ignore it.
	// Additive in Stage 1 (nothing sets it until the orchestrator becomes a pure
	// driver in Stage 3).
	Spec *phasespec.PhaseSpec `json:"spec,omitempty"`

	// UpstreamSignals is the accumulating signal bus piped in from prior phases,
	// namespaced <phase>.<key> (e.g. "build.files_touched"). A phase keys its
	// own routing/classify decisions off these without re-reading artifacts.
	UpstreamSignals map[string]any `json:"upstream_signals,omitempty"`

	// ModelRoutingCLI/ModelRoutingTier (cycle-440 MR4a/c) carry the whole-cycle
	// plan's clamped {cli,tier} proposal for THIS phase — but ONLY when
	// config.ModelRouting==ModelRoutingAuto (advisory computes and logs the
	// same clamped proposal to phase-plan.json but leaves these empty; static
	// never sets them). The runner applies them as a SOFT dispatch overlay
	// (llmroute.ApplySoftOverlay): promote to chain primary without discarding
	// the profile's fallback chain, so a benched/failing proposal still falls
	// back via the ordinary cli-health chain. Empty on every pre-cycle-440
	// dispatch and on any mode other than auto — omitempty keeps a zero-value
	// PhaseRequest's JSON shape byte-identical.
	ModelRoutingCLI  string `json:"model_routing_cli,omitempty"`
	ModelRoutingTier string `json:"model_routing_tier,omitempty"`
}

// PhaseResponse is the output envelope from PhaseRunner.Run.
type PhaseResponse struct {
	Phase        string     `json:"phase"`
	Verdict      string     `json:"verdict"`
	ArtifactsDir string     `json:"artifacts_dir"`
	NextPhase    string     `json:"next_phase,omitempty"`
	CostUSD      float64    `json:"cost_usd"`
	Tokens       TokenUsage `json:"tokens"`
	DurationMS   int64      `json:"duration_ms"`
	// BootMS is the cold REPL-boot latency carried up from the bridge
	// (ADR-0043 A0) — the slice of DurationMS that was pure dispatch overhead
	// (tmux new-session → prompt marker), not model think time. 0 = no cold boot.
	BootMS      int64        `json:"boot_ms,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`

	// Signals is the namespaced signal map this phase emits onto the bus — the
	// typed "message on the pipe." The router (Stage 3) consumes these directly
	// instead of re-parsing markdown. Keys are <phase>.<key>. Additive in
	// Stage 1: emitted but not yet read by routing.
	Signals map[string]any `json:"signals,omitempty"`

	// CommitSHA is the commit that anchors this phase's deliverable
	// (ADR-0027 commit-as-evidence). Empty when the phase committed nothing.
	CommitSHA string `json:"commit_sha,omitempty"`

	// Reconciled is set when the bridge reported a process failure
	// (ErrArtifactTimeout) but the agent's deliverable was on disk, well-formed,
	// and gate-passing — so the runner trusted the deliverable's verdict instead
	// of synthesizing FAIL. Surfaced for the auditable self-healing trail
	// (orchestrator appends a reconciled_timeout ledger entry).
	Reconciled bool `json:"reconciled,omitempty"`

	// ModelSource + ResolvedModel (T3, cycle-463) record WHICH resolution path
	// won this phase's dispatch — "profile" (neither pin nor advisor overlay),
	// "pin" (an operator policy.json pin, which always wins), or "advisor" (the
	// MR4c soft overlay applied) — plus the concrete resolved model/tier. Closes
	// the P3 observability gap: per-phase model provenance was previously
	// unrecorded, so a dormant advisor overlay had no artifact proving it.
	ModelSource   string `json:"model_source,omitempty"`
	ResolvedModel string `json:"resolved_model,omitempty"`
}

// PhaseRunner runs a single phase. The orchestrator never knows which
// runner impl is in play (in-process vs subprocess) — that's the
// independence guarantee for Approach C.
type PhaseRunner interface {
	Name() string
	Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error)
}
