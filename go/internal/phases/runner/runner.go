// Package runner provides BaseRunner, a Template Method implementation
// of the shared phase-dispatch skeleton. Each subagent-dispatching phase
// (intent, scout, triage, tdd, build, audit) supplies a tiny Hooks
// implementation; BaseRunner orchestrates the identical surrounding
// logic — profile lookup, prompt composition, bridge dispatch, artifact
// reading, classification, response packaging.
//
// Pattern: Template Method (GoF). The "template" is BaseRunner.Run; the
// "primitive operations" are the Hooks methods. Phases override the
// variation points without touching the dispatch shape.
//
// Goals:
//
//   - DRY: collapse ~70 LoC of identical boilerplate per phase to ~5
//   - SRP: each Hooks method does one thing (compose, classify, etc.)
//   - Test-stability: the existing per-phase integration tests assert
//     the same external contract (BridgeRequest shape, PhaseResponse
//     fields), so they keep passing across the refactor — the
//     behavior-preservation harness called out in the plan.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Hooks captures the per-phase variation points BaseRunner delegates
// to. Implementations are typically small value types embedded in each
// phase package's Phase struct.
type Hooks interface {
	// PhaseName returns the canonical phase identifier ("build",
	// "scout", "audit"). Used for phaseflags lookup, profile-file name,
	// BridgeRequest.Agent, and model env-var key.
	PhaseName() string

	// AgentPromptName returns the agent doc to load via
	// prompts.Loader.Agent (e.g., "evolve-builder" for the build
	// phase). Differs from PhaseName because agent docs historically
	// carry the "evolve-" prefix.
	AgentPromptName() string

	// ArtifactFilename returns the artifact the agent is contracted to
	// produce, joined with req.Workspace. Takes req so phases can vary
	// the filename per-request (e.g., intent's delta mode chooses
	// "intent-delta.md" instead of "intent.md").
	ArtifactFilename(req core.PhaseRequest) string

	// DefaultModel returns the model identifier to use when
	// EVOLVE_<PHASE>_MODEL is unset. Most phases use "auto"; audit
	// uses "opus" for adversarial cross-family diversity.
	DefaultModel() string

	// ComposePrompt assembles the final prompt sent to the bridge. The
	// agent doc body comes pre-loaded; phases typically append a cycle
	// context block.
	ComposePrompt(agentBody string, req core.PhaseRequest) string

	// Classify inspects the artifact (file contents or stdout) and
	// returns the phase's verdict, any diagnostics, and the next phase
	// name. BaseRunner handles bridge-error and missing-artifact paths
	// before calling Classify; this method only runs on the success
	// branch.
	Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (verdict string, diagnostics []core.Diagnostic, nextPhase string)
}

// Skipper is an optional Hooks extension. When a Hooks implementation
// also satisfies Skipper, BaseRunner consults ShouldSkip before any
// bridge call. If skipped is true, BaseRunner returns a SKIPPED
// PhaseResponse with the supplied verdict and nextPhase and never
// touches the bridge. Used by triage (EVOLVE_TRIAGE_DISABLE), tdd
// (EVOLVE_TEST_PHASE_ENABLED=0), and retro (previous verdict guard).
//
// Why optional? Most phases never skip. Forcing every Hooks impl to
// implement a no-op ShouldSkip violates ISP (interface-segregation).
type Skipper interface {
	ShouldSkip(req core.PhaseRequest) (skipped bool, verdict, nextPhase string, diags []core.Diagnostic)
}

// InlinePromptProvider is an optional Hooks extension. When a Hooks
// implementation also satisfies it AND returns ok=true, BaseRunner composes
// the prompt from the supplied in-band body and never reads
// agents/<AgentPromptName>.md. Returning ("", false) — or not implementing
// this interface at all — preserves the legacy disk-load path byte-for-byte.
//
// Used by minted/spec phases (specrunner) that ship their prompt as data
// (no file on disk). Optional for the same ISP reason as Skipper: built-in
// phases load their agent docs from disk and must not be forced to implement
// a no-op.
// SecondaryArtifactsProvider is an OPTIONAL hook: phases whose contract
// requires deliverables beyond the primary artifact (audit on continuation
// cycles: defect-dispositions.json) return their absolute paths; the bridge
// completion detector holds phase-complete until each exists (Phase B).
type SecondaryArtifactsProvider interface {
	SecondaryArtifacts(req core.PhaseRequest) []string
}

// secondaryArtifacts resolves the optional hook; nil for phases without it.
func secondaryArtifacts(h Hooks, req core.PhaseRequest) []string {
	if sp, ok := h.(SecondaryArtifactsProvider); ok {
		return sp.SecondaryArtifacts(req)
	}
	return nil
}

type InlinePromptProvider interface {
	InlinePromptBody() (string, bool)
}

// Options is the BaseRunner constructor envelope. Bridge and Prompts
// are required; NowFn defaults to time.Now.
type Options struct {
	Hooks   Hooks
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// ResolveLLM is the seam for resolving the "auto" model sentinel.
	// When nil, defaults to resolvellm.Resolve.
	ResolveLLM func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	// StdoutFilter is the seam for the post-phase .clean.txt writer.
	// When nil, defaults to logfilter.Process. Per-instance field (not a
	// package global) keeps t.Parallel() tests race-free.
	StdoutFilter func(workspace, phase string) error
	// EventsProducer is the seam for the post-phase <phase>-events.ndjson
	// writer (ADR-0020). When nil, defaults to phasestream.Produce. Unlike
	// StdoutFilter this is load-bearing: cyclecost + cycleclassify read the
	// events stream, so it is always-on (no disable flag). prompt is the
	// composed phase prompt, threaded to the Classifier's echo-veto
	// (ProduceConfig.InjectedPrompt, cycle-672) so agent-quoted prompt text
	// never classifies infra_failure.
	EventsProducer func(workspace, phase, cli string, cycle int, prompt string) error
	// Optional marks this phase as non-essential to the cycle. When true, a
	// bridge ErrArtifactTimeout degrades to a WARN that lets the cycle
	// advance (the state machine's successor is verdict-unconditional for
	// optional phases like build-planner) instead of aborting. Set by the
	// owning phase (e.g. buildplanner.New). Default false = hard-fail, the
	// historical behavior for mandatory phases. See Workstream D / cycle-120.
	Optional bool
	// VerifyFn is the seam for the deliverable well-formedness check used by
	// reconcile-on-timeout: when the bridge reports ErrArtifactTimeout but the
	// agent's contracted deliverable is on disk and well-formed, the runner
	// trusts the deliverable's verdict instead of synthesizing FAIL. When nil,
	// defaults to deliverable.Verify. Per-instance (not a package global) so
	// t.Parallel() tests stay race-free, mirroring StdoutFilter.
	VerifyFn func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	// ContractVerifier is the deliverables gate's own verifier offered to the
	// verdict engine so there is ONE verifier: the bytes the engine classifies
	// are the bytes the gate will approve (a sole recoverable bad_verdict is
	// salvaged, persisted and reported before classification — cycle 1685
	// sealed FAIL on the unrepaired bytes while the gate approved the
	// repaired file). The composition root injects the same Reviewer it
	// appends to the orchestrator's reviewers; it is an accessor because the
	// Reviewer is built after the runners (it needs the merged phase
	// catalog) — the Signals precedent. nil, or an accessor returning nil
	// (tests, gate off), falls back to the catalog-aware verify. VerifyFn
	// (tests) outranks it.
	ContractVerifier func() ContractVerifier
	// HostEffects performs the phase's declared host effects before the verdict
	// engine judges it; an accessor for the same reason as ContractVerifier.
	HostEffects func() core.HostEffects
	// SleepFn is the seam for the delay between the verdict engine's bounded
	// settle-retry attempts (verdict.Engine.settle — see its doc for the
	// cycles 824/825 rationale). When nil, defaults to settleSleep (time.Sleep).
	// Per-instance so t.Parallel() tests can inject a no-op for determinism,
	// mirroring NowFn.
	SleepFn func(time.Duration)
	// PhaseIO is the EVOLVE_PHASE_IO rollout stage (ADR-0050 §3.10). When VerifyFn
	// is nil, it is threaded into the catalog-aware reconcile default so the
	// reconcile-on-timeout rung honors the same stage-gated failure-context
	// requirement as the host gate. Zero value (StageOff) keeps every existing
	// Options{} literal byte-identical — only build/scout/triage set it (the
	// phases with a RequireFailureContextPhaseIO contract).
	PhaseIO config.Stage
	// CompactPrompts, when true, strips on-demand reference sections from
	// disk-loaded agent docs before ComposePrompt. Replaces the former
	// EVOLVE_COMPACT_PROMPTS env read. Inline bodies (minted/spec phases) are
	// never stripped regardless of this setting (R7).
	CompactPrompts bool
	// DisableStdoutFilter, when true, skips the post-phase .clean.txt writer.
	// Replaces the former EVOLVE_STDOUT_FILTER=off env check. Default false =
	// filter enabled, matching the historical "on" default.
	DisableStdoutFilter bool
	// UniversalFallback (workflow.universal_fallback, default true) enables the
	// last-resort dispatch tier: when a phase's whole configured CLI chain has no
	// binary on this host, DiscoverCLIsFn's installed+authed CLIs (family-filtered
	// by the profile allowlist) are appended so the loop routes to whatever LLM is
	// present instead of halting. No-op unless DiscoverCLIsFn is also wired.
	UniversalFallback bool
	// DiscoverCLIsFn is the memoized system-CLI discovery seam (composition root
	// closes it over bridge.Doctor: installed + authed + non-blocked driver names,
	// e.g. "agy-tmux"). nil ⇒ universal fallback is inert (byte-identical to the
	// pre-feature dispatch). Kept a seam so the runner package never imports bridge.
	DiscoverCLIsFn func() []string
	// Diag is the injectable diagnostics logger (T3, cycle-463): the MR4c
	// advisor-overlay observability lines route through it so a test can
	// capture them instead of the global log.Diag() stderr sink. Zero value
	// (both sinks nil) defaults to log.Diag() — production behavior is
	// unchanged.
	Diag log.Console
	// Signals is the Signal Center accessor the verdict engine reports through
	// (ADR-0103 unit 11). Nil ⇒ New adopts the Center the injected Bridge
	// carries when it exposes one (the production Adapter), else the Null
	// Object. Explicit so tests and foreign roots can inject one.
	Signals func() *signalcenter.Center
}

// BaseRunner is the Template Method implementation. Construct one per
// phase via New(); use it as a core.PhaseRunner.
// ContractVerifier is what the deliverables gate offers the verdict engine
// (deliverable.Reviewer implements it): the gate's verification, salvage
// included, keyed by the dispatch identity so a salvage is reported under its
// cycle. Defined here, at the consumer.
type ContractVerifier interface {
	VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (deliverable.Result, error)
}

type BaseRunner struct {
	hooks             Hooks
	bridge            core.Bridge
	prompts           *prompts.Loader
	nowFn             func() time.Time
	resolveLLM        func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	eventsProducer    func(workspace, phase, cli string, cycle int, prompt string) error
	compactPrompts    bool
	universalFallback bool
	discoverCLIsFn    func() []string
	diag              log.Console
	// judge is unit 11's (ADR-0103): the verdict engine, built ONCE by New over
	// the resolved probe, clock, stdout filter, optional flag and Center
	// accessor — which live in the engine only (wiredVerdictEngine).
	contractVerifier func() ContractVerifier
	hostEffects      func() core.HostEffects
	signals          func() *signalcenter.Center
	verifyInjected   bool
	judge            *verdict.Engine
}

// New constructs a BaseRunner. Panics if Hooks is nil — that's a
// programmer error caught at startup, not a runtime condition.
func New(opts Options) *BaseRunner {
	if opts.Hooks == nil {
		panic("phases/runner: Hooks required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	resolveLLM := opts.ResolveLLM
	if resolveLLM == nil {
		resolveLLM = resolvellm.Resolve
	}
	eventsProducer := opts.EventsProducer
	if eventsProducer == nil {
		eventsProducer = func(workspace, phase, cli string, cycle int, prompt string) error {
			return phasestream.Produce(phasestream.ProduceConfig{
				Workspace: workspace, Phase: phase, CLI: cli, Cycle: cycle,
				InjectedPrompt: prompt,
			})
		}
	}
	diag := opts.Diag
	if diag.Out == nil && diag.Err == nil {
		diag = log.Diag()
	}
	// Universal-fallback defaults: per-instance Options win (test injection);
	// otherwise fall back to the composition-root package seams (set once in
	// cmd_cycle.go from workflow.universal_fallback + a memoized bridge.Doctor
	// closure — same set-once pattern as PhaseBoundaryCheckpointer, so the ~10
	// per-phase constructors need not each thread the discovery closure). Default
	// zero values ⇒ inert, byte-identical to the pre-feature dispatch.
	universalFallback := opts.UniversalFallback || DefaultUniversalFallback
	discoverCLIsFn := opts.DiscoverCLIsFn
	if discoverCLIsFn == nil {
		discoverCLIsFn = DefaultDiscoverCLIsFn
	}
	b := &BaseRunner{
		hooks:             opts.Hooks,
		bridge:            opts.Bridge,
		prompts:           opts.Prompts,
		nowFn:             nowFn,
		resolveLLM:        resolveLLM,
		eventsProducer:    eventsProducer,
		compactPrompts:    opts.CompactPrompts,
		universalFallback: universalFallback,
		discoverCLIsFn:    discoverCLIsFn,
		diag:              diag,
	}
	b.judge = wiredVerdictEngine(opts)
	b.contractVerifier = opts.ContractVerifier
	b.hostEffects = opts.HostEffects
	b.signals = resolveSignals(opts)
	b.verifyInjected = opts.VerifyFn != nil
	return b
}

// settleSleep is the process clock for the verdict engine's settle ladder
// (the default behind Options.SleepFn, handed to the engine as WithSleep) —
// time.Sleep in production; a test init flips it to a no-op so the settle
// window costs zero wall-clock in the package's test suite (the retry sits on
// the common clean-exit path, so real sleeps would balloon package test time).
var settleSleep = time.Sleep

// Name implements core.PhaseRunner.
func (b *BaseRunner) Name() string { return b.hooks.PhaseName() }

// Run implements core.PhaseRunner. The template:
//
//  1. validate deps (bridge, prompts), load the agent prompt body and compose
//     the final prompt via the hooks; snapshot the artifact pre-dispatch
//  2. resolve the dispatch plan (policy pin / profile / advisor overlay, the
//     CLI chain, the tier)
//  3. dispatch through the bridge across the fallback chain, inside the
//     worktree fence, then perform the phase's declared host effects
//  4. judge the outcome through the verdict engine (ADR-0103 unit 11): the
//     bounded settle ladder, the teardown reconcile arms, the verdict-source
//     rule (the contracted file, never the pane), Classify via hook, the ship
//     guard, the response
//
// Bridge errors and missing-prompts errors short-circuit to a FAIL
// response with the error attached as a diagnostic.
func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	req.WorktreeVerified = false
	prep, early, err := b.preparePhaseExecution(req)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	if early != nil {
		return *early, nil
	}

	dispatchPlan, early, err := b.resolveDispatchPlan(req, prep)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	dispatchResult := b.dispatchPhaseAttempts(ctx, req, prep, dispatchPlan)
	req.WorktreeVerified = dispatchResult.worktreeVerified

	d := dispatchOf(req, prep, dispatchPlan, dispatchResult)
	b.performHostEffects(ctx, d)
	return b.judge.Judge(ctx, d, b.classifyWith(req, dispatchResult.bridgeResponse))
}

func (b *BaseRunner) withExplanationContract(body, phase string) (string, error) {
	var agentName, heading string
	switch phase {
	case string(core.PhaseBuild):
		agentName = "evolve-builder-reference"
		heading = "## Section: explanation-documentation-contract"
	case string(core.PhaseAudit):
		agentName = "evolve-auditor-reference"
		heading = "## Section: explanation-documentation-review"
	default:
		return body, nil
	}
	if strings.Contains(body, heading) {
		return body, nil
	}
	agent, err := b.prompts.Agent(agentName)
	if err != nil {
		return "", err
	}
	start := strings.Index(agent.Body, heading)
	if start < 0 {
		return "", fmt.Errorf("%s is missing %q", agentName, heading)
	}
	section := agent.Body[start:]
	if next := strings.Index(section[len(heading):], "\n## Section:"); next >= 0 {
		section = section[:len(heading)+next]
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.TrimSpace(section) + "\n", nil
}

func requiresExplanationSandbox(phase string, req core.PhaseRequest) bool {
	return phase == string(core.PhaseBuild) && req.ExplanationDocumentationVersion != 0
}

// cycleContextBoundary is the single canonical marker that separates a phase
// prompt's cache-stable static prefix (persona/rules/agent-doc body) from its
// per-cycle dynamic tail. BaseCycleContext writes it and StaticPrefix splits on
// it — one literal, so the two can never drift apart and silently bust the
// provider prompt-cache.
const cycleContextBoundary = "\n\n## Cycle Context\n"

// BaseCycleContext returns the canonical "## Cycle Context" block shared by
// every phase that uses BaseRunner. It writes body, then the four mandatory
// fields (cycle, goal_hash, project_root, workspace). Phase-specific extras
// (worktree, goal, mode, carryover_summary, etc.) are the caller's responsibility
// — they append them after this call so the base block stays the single source.
func BaseCycleContext(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString(cycleContextBoundary)
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if recalled := req.Context[core.CtxKeyRecallMemory]; recalled != "" {
		fmt.Fprintf(&b, "- recalled_lessons_untrusted_json: %q\n", recalled)
	}
	AppendExplanationContext(&b, req)
	return b.String()
}

// AppendExplanationContext writes the complete verified Build-explanation
// handoff as a single-line JSON data field plus stable convenience fields.
// Audit and Retro share this serializer so their prompts cannot drift from the
// evidence their deterministic report validators require.
func AppendExplanationContext(b *strings.Builder, req core.PhaseRequest) {
	if req.ExplanationDocumentationVersion != 0 {
		fmt.Fprintf(b, "- explanation_documentation_version: %d\n", req.ExplanationDocumentationVersion)
	}
	if req.BuildExplanationState != "" {
		fmt.Fprintf(b, "- explanation_handoff_state: %s\n", req.BuildExplanationState)
	}
	if req.BuildExplanationError != "" {
		fmt.Fprintf(b, "- explanation_error_untrusted_json: %q\n", req.BuildExplanationError)
	}
	if explanation := req.BuildExplanation; explanation != nil {
		fmt.Fprintf(b, "- explanation_status: %s\n", explanation.Status)
		fmt.Fprintf(b, "- explanation_contract_version: %d\n", explanation.ContractVersion)
		fmt.Fprintf(b, "- explanation_document: %s (sha256:%s)\n", explanation.DocumentPath, explanation.DocumentSHA256)
		if encoded, err := json.Marshal(explanation); err == nil {
			fmt.Fprintf(b, "- explanation_handoff_untrusted_json: %s\n", encoded)
		}
	}
}

// StaticPrefix returns the cache-stable prefix of a composed phase prompt:
// everything before the canonical "## Cycle Context" boundary that
// BaseCycleContext emits. Provider prompt-caches key on this byte-identical
// prefix, so isolating it lets callers verify (and pin, via the cache-stable
// audit) that no per-cycle dynamic value — cycle number, goal_hash, workspace —
// drifts above the boundary. When the boundary is absent the whole prompt is
// the prefix.
func StaticPrefix(prompt string) string {
	if i := strings.Index(prompt, cycleContextBoundary); i >= 0 {
		return prompt[:i]
	}
	return prompt
}

// FormatDigestShadowLog renders the role projection's dispatch-time evidence.
func FormatDigestShadowLog(phase string, record digest.ShadowRecord) string {
	return fmt.Sprintf("[runner] phase=%s digest-shadow outcome=%s full_bytes=%d digest_bytes=%d parity=%t",
		phase, record.Outcome, record.FullBytes, record.DigestBytes, record.Parity)
}

// ComposePrompt exposes the phase's prompt assembly (Hooks.ComposePrompt) as a
// public seam on BaseRunner so a caller can compose a prompt without launching
// the bridge — used by the cache-stable-prefix audit to inspect the static
// prefix for every BaseRunner-based phase. Run uses the same hook internally
// (see below); this method adds reach, not behavior.
func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}

// PersonaAvailable implements core.PersonaProber: it resolves the persona doc
// exactly as Run does (same loader, same name, same inline-prompt exemption)
// without dispatching anything, so the planner can exclude a phase whose doc
// does not exist instead of discovering it one dispatch, one skip and one
// retrospective later.
func (b *BaseRunner) PersonaAvailable() error {
	if ip, ok := b.hooks.(InlinePromptProvider); ok {
		if _, inline := ip.InlinePromptBody(); inline {
			return nil
		}
	}
	if b.prompts == nil {
		return fmt.Errorf("%s: prompts loader required", b.hooks.PhaseName())
	}
	if _, err := b.prompts.Agent(b.hooks.AgentPromptName()); err != nil {
		if errors.Is(err, fs.ErrNotExist) && !errors.Is(err, prompts.ErrNoSource) {
			return fmt.Errorf("%s: load agent: %w: %w", b.hooks.PhaseName(), core.ErrAgentDocMissing, err)
		}
		return fmt.Errorf("%s: load agent: %w", b.hooks.PhaseName(), err)
	}
	return nil
}
