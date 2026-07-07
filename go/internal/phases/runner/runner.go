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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/systemprompt"
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
	// events stream, so it is always-on (no disable flag).
	EventsProducer func(workspace, phase, cli string, cycle int) error
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
	// Diag is the injectable diagnostics logger (T3, cycle-463): the MR4c
	// advisor-overlay observability lines route through it so a test can
	// capture them instead of the global log.Diag() stderr sink. Zero value
	// (both sinks nil) defaults to log.Diag() — production behavior is
	// unchanged.
	Diag log.Console
}

// BaseRunner is the Template Method implementation. Construct one per
// phase via New(); use it as a core.PhaseRunner.
type BaseRunner struct {
	hooks               Hooks
	bridge              core.Bridge
	prompts             *prompts.Loader
	nowFn               func() time.Time
	resolveLLM          func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	stdoutFilter        func(workspace, phase string) error
	eventsProducer      func(workspace, phase, cli string, cycle int) error
	optional            bool
	verifyFn            func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	compactPrompts      bool
	disableStdoutFilter bool
	diag                log.Console
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
	stdoutFilter := opts.StdoutFilter
	if stdoutFilter == nil {
		stdoutFilter = logfilter.Process
	}
	eventsProducer := opts.EventsProducer
	if eventsProducer == nil {
		eventsProducer = func(workspace, phase, cli string, cycle int) error {
			return phasestream.Produce(phasestream.ProduceConfig{
				Workspace: workspace, Phase: phase, CLI: cli, Cycle: cycle,
			})
		}
	}
	verifyFn := opts.VerifyFn
	if verifyFn == nil {
		// Catalog-aware so the reconcile check resolves user/minted phases
		// under the SAME policy as the host gate and the agent self-check —
		// a builtin-only default left an inserted phase's surviving artifact
		// unresolvable on timeout, synthesizing FAIL. Stage-threaded (3.10
		// Slice 1) so the rung also reaches the host gate's verdict at enforce;
		// opts.PhaseIO's zero value (StageOff) is byte-identical to the prior
		// VerifyCatalogAware default.
		stage := opts.PhaseIO
		verifyFn = func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.VerifyCatalogAwareStage(phase, roots, stage)
		}
	}
	diag := opts.Diag
	if diag.Out == nil && diag.Err == nil {
		diag = log.Diag()
	}
	return &BaseRunner{
		hooks:               opts.Hooks,
		bridge:              opts.Bridge,
		prompts:             opts.Prompts,
		nowFn:               nowFn,
		resolveLLM:          resolveLLM,
		stdoutFilter:        stdoutFilter,
		eventsProducer:      eventsProducer,
		optional:            opts.Optional,
		verifyFn:            verifyFn,
		compactPrompts:      opts.CompactPrompts,
		disableStdoutFilter: opts.DisableStdoutFilter,
		diag:                diag,
	}
}

// Name implements core.PhaseRunner.
func (b *BaseRunner) Name() string { return b.hooks.PhaseName() }

// Run implements core.PhaseRunner. The template:
//
//  1. validate deps (bridge, prompts)
//  2. load agent prompt body
//  3. compose final prompt via hook
//  4. resolve cli / model / extraFlags from env-chain + profile
//  5. dispatch bridge.Launch
//  6. read artifact (stdout, then file fallback)
//  7. classify via hook
//  8. package PhaseResponse
//
// Bridge errors and missing-prompts errors short-circuit to a FAIL
// response with the error attached as a diagnostic.
func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := b.nowFn()
	phase := b.hooks.PhaseName()

	if b.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("%s: bridge required", phase)
	}
	if b.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("%s: prompts loader required", phase)
	}

	// Optional pre-bridge skip predicate. ISP: only phases that opt
	// into Skipper get consulted; the rest skip this branch entirely.
	if skipper, ok := b.hooks.(Skipper); ok {
		if skipped, verdict, nextPhase, diags := skipper.ShouldSkip(req); skipped {
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      verdict,
				ArtifactsDir: req.Workspace,
				NextPhase:    nextPhase,
				DurationMS:   b.nowFn().Sub(start).Milliseconds(),
				Diagnostics:  diags,
			}, nil
		}
	}

	// Inline body wins over disk-load, keyed on the provider's ok flag (not
	// body emptiness). Only phases that opt into InlinePromptProvider are
	// consulted; see its godoc for the ISP rationale.
	body, inline := "", false
	if ip, ok := b.hooks.(InlinePromptProvider); ok {
		body, inline = ip.InlinePromptBody()
	}
	if !inline {
		agent, err := b.prompts.Agent(b.hooks.AgentPromptName())
		if err != nil {
			return core.PhaseResponse{}, fmt.Errorf("%s: load agent: %w", phase, err)
		}
		body = agent.Body
		if b.compactPrompts {
			body = prompts.StripOnDemandSections(body)
		}
	}

	prompt := b.hooks.ComposePrompt(body, req)
	artifactPath := filepath.Join(req.Workspace, b.hooks.ArtifactFilename(req))
	profileDir := filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	// Profile JSON files use the AGENT name (e.g., tdd-engineer.json,
	// builder.json, auditor.json, retrospective.json) — NOT the phase
	// name (tdd, build, audit, retro). The convention is "strip the
	// 'evolve-' prefix from AgentPromptName". Source: cycle 106
	// (2026-05-25) integration smoke where phase=tdd looked for
	// `.evolve/profiles/tdd.json` (which doesn't exist) instead of
	// `.evolve/profiles/tdd-engineer.json` (which does). Also matches
	// CLAUDE.md's `EVOLVE_<AGENT>_<KEY>` env-var convention so the
	// phaseflags env-key generation aligns with the documented
	// `EVOLVE_TDD_ENGINEER_PERMISSION_MODE` (not `EVOLVE_TDD_*`).
	profileName := strings.TrimPrefix(b.hooks.AgentPromptName(), "evolve-")
	profilePath := filepath.Join(profileDir, profileName+".json")

	// CLI resolution chain: EVOLVE_CLI env var > profile.cli > default
	// "claude-p". Before this fix the runner only consulted EVOLVE_CLI
	// and defaulted to claude-p regardless of profile.cli, which meant
	// operators editing .evolve/profiles/<agent>.json:cli to switch a
	// phase to codex or agy had no effect — the runner silently
	// dispatched to claude-p anyway.
	// Source: cycle 107 (2026-05-25) attempted-codex smoke that
	// produced claude-sonnet-4-6 output despite cli=codex in every
	// profile. Operator misread "delegation" because the resolved CLI
	// wasn't logged.
	var prof *profiles.Profile
	if loader := profiles.NewFromDir(profileDir); loader != nil {
		if p, err := loader.Get(profileName); err == nil {
			prof = &p
		} else if st, statErr := os.Stat(profileDir); statErr == nil && st.IsDir() {
			msg := fmt.Sprintf("profile not found: %s", profilePath)
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
			}, fmt.Errorf("%s: %s: %w", phase, msg, err)
		}
	}

	// Advisory turn-budget hint: when a profile declares turn_budget_hint, append
	// a non-binding budget note so the agent self-limits (prioritize breadth,
	// finalize once completion gates are met). Purely advisory — the hard stops
	// remain max_turns + the artifact timeout. Activates the otherwise-dormant
	// profiles.Profile.TurnBudgetHint field (declared in ~8 profiles but never
	// consumed before this).
	if prof != nil && prof.TurnBudgetHint > 0 {
		prompt += fmt.Sprintf("\n\n## Budget\nAdvisory turn budget for this phase: ~%d turns. Prioritize breadth over depth; write your report as soon as the completion gates are satisfied.\n", prof.TurnBudgetHint)
	}

	// Challenge-token injection (cycle-269): the bash→Go migration dropped
	// the prompt-side half of the proof-of-read protocol — builders were
	// never TOLD to echo the minted token, so compliance depended on an
	// agent spontaneously reading scout-report line 2 (the claude fallback
	// didn't; a perfect build FAILed at audit). Contract-driven (the same
	// SSOT the deliverable gate checks), deterministic, and absent-token ⇒
	// byte-identical prompt.
	if c, ok := phasecontract.For(phase); ok && c.RequireChallengeToken {
		if tok, terr := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); terr == nil {
			if t := strings.TrimSpace(string(tok)); t != "" {
				prompt += fmt.Sprintf("\n\n## Challenge Token (proof-of-read — MANDATORY)\nCopy this token verbatim into your report as an HTML comment near the top: <!-- challenge-token: %s -->\nA report without it is rejected and re-dispatched.\n", t)
			}
		}
	}

	// User-controlled policy pin (absolute): a pinned CLI/model for this phase
	// overrides env/profile/default resolution. Keyed by phase name. Validated
	// against the profile guardrails (allowed_clis + model_tier_envelope) — an
	// out-of-bounds pin hard-fails the phase loudly rather than silently
	// breaching the trust-kernel constraints. Escape hatch: --bypass-policy flag.
	var pin *policy.Pin
	if !req.BypassPolicy {
		pol, perr := policy.Load(filepath.Join(req.ProjectRoot, ".evolve", "policy.json"))
		if perr != nil {
			// Malformed policy must fail loudly, not silently ignore user rules.
			return core.PhaseResponse{
				Phase: phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
				Diagnostics: []core.Diagnostic{{Severity: "error", Message: perr.Error()}},
			}, fmt.Errorf("%s: %w", phase, perr)
		}
		if p, ok := pol.PinFor(phase); ok {
			if verr := policy.ValidatePin(phase, p, prof); verr != nil {
				return core.PhaseResponse{
					Phase: phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
					Diagnostics: []core.Diagnostic{{Severity: "error", Message: verr.Error()}},
				}, fmt.Errorf("%s: %w", phase, verr)
			}
			pin = &p
			log.Diag().Infof("[runner] phase=%s policy pin: cli=%q model=%q\n", phase, p.CLI, p.Model)
		}
	}

	// Single dispatch resolver (llmroute): one Plan carries the CLI fallback
	// chain AND the resolved model, so there is exactly one place that decides
	// "which CLI + model runs this phase". Precedence (preserved verbatim):
	//   CLI:   EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile.cli > "claude-tmux",
	//          then + profile.cli_fallback (deduped); triggers default
	//          {80,81,124,127}. A single-element chain is byte-identical to
	//          pre-fallback behavior.
	//   model: EVOLVE_<AGENT>_MODEL > profile.model_tier_default >
	//          Hooks.DefaultModel(), then "auto" → autoExpand.
	// The model env key is AGENT-keyed (EVOLVE_<PROFILE_NAME>_MODEL), matching
	// cmd_loop's `--model <agent>=X` writer + the PERMISSION_MODE resolver below.
	//
	// autoExpand bridges the resolvellm seam so "auto" expansion stays
	// byte-identical (keyed by `phase`, NOT profileName — preserved from the
	// pre-Step-9 behavior when the now-removed llm_config layer was phase-keyed).
	// claude -p rejects a literal "auto" (HTTP 404), so this MUST resolve before
	// dispatch. The CLI the seam computes is intentionally NOT used for dispatch —
	// the chain above is authoritative.
	autoExpand := func(role string) (string, bool) {
		res, err := b.resolveLLM(role, resolvellm.Options{})
		if err != nil {
			return "", false
		}
		if res.ModelTier != "" {
			return res.ModelTier, true
		}
		return "", false
	}
	plan := llmroute.Resolve(profileName, phase, b.hooks.DefaultModel(), req.Env, prof, autoExpand, pin)
	// Soft dispatch overlay (cycle-440 MR4c): an advisor-proposed {cli,tier}
	// threaded via PhaseRequest.ModelRoutingCLI/Tier (already clamped upstream
	// by router.ClampPlanModelRouting under model_routing=auto) promotes to
	// chain PRIMARY without discarding the profile's fallback chain — unlike an
	// absolute policy.Pin, a benched overlay CLI still falls back via the
	// capability-probe + cli-health bench passes below. A policy pin always
	// wins (soft overlay never applies alongside one); zero overlay fields are
	// a byte-identical noop.
	overlayProposed := req.ModelRoutingCLI != "" || req.ModelRoutingTier != ""
	modelSource := "profile"
	switch {
	case pin != nil:
		modelSource = "pin"
	case overlayProposed:
		modelSource = "advisor"
	}
	if pin == nil && overlayProposed {
		plan = llmroute.ApplySoftOverlay(plan, llmroute.Overlay{CLI: req.ModelRoutingCLI, Tier: req.ModelRoutingTier})
		b.diag.Infof("[runner] phase=%s advisor overlay cli=%s tier=%s\n", phase, req.ModelRoutingCLI, req.ModelRoutingTier)
	} else if pin == nil {
		b.diag.Infof("[runner] phase=%s no advisor overlay (profile default)\n", phase)
	}
	// Capability probe: demote (don't delete) candidates whose binary isn't on
	// PATH so a missing CLI doesn't burn a 60s boot timeout before the chain
	// advances. Log the reorder inline with the dispatch log.
	//
	// SKIPPED when a CLI is policy-pinned: the probe reorders by binary
	// availability, which would silently demote a pinned-but-missing CLI out of
	// the primary slot — violating the "policy pin is absolute" contract. A
	// pinned CLI is attempted as-is; if its binary is absent the dispatch
	// surfaces a real ExitMissingBinary (127), which the profile fallback chain
	// can still recover from via the normal trigger path.
	if pin == nil || pin.CLI == "" {
		preCandidates := plan.Candidates
		plan = llmroute.Probe(plan, nil)
		if !sameCandidates(preCandidates, plan.Candidates) {
			log.Diag().Infof("[runner] phase=%s capability probe reordered chain: %v -> %v\n",
				phase, preCandidates, plan.Candidates)
		}
	}
	// CLI-health bench: demote families with an ACTIVE bench (classified
	// transient wall, e.g. rate_limit) so the chain starts at a healthy CLI
	// instead of re-burning the walled primary's boot window (cycle-283).
	// Same pin bypass as the capability probe; lazy expiry inside.
	plan = b.applyBenchToPlan(req.ProjectRoot, phase, plan, pin != nil && pin.CLI != "", req.Env)
	cli := plan.Candidates[0]
	// Disambiguating dispatch log: tells observers which CLI is actually being
	// invoked and why (an output stream saying `model: claude-sonnet-4-6` could
	// otherwise be misread as "codex delegating to claude").
	if len(plan.Candidates) > 1 {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s fallback=%v triggers=%v\n",
			phase, profileName, cli, plan.PrimarySource, profilePath, plan.Candidates[1:], plan.Triggers)
	} else {
		log.Diag().Infof("[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
			phase, profileName, cli, plan.PrimarySource, profilePath)
	}
	model := plan.Model
	// Per-phase permission mode comes from the request snapshot first, then
	// the typed agent profile. The process environment is intentionally not
	// consulted: profiles are the persistent SSOT and req.Env is the explicit
	// per-dispatch override surface. Passed as typed config — the bridge
	// realizes it per-CLI via the LaunchIntent (no raw flag leak).
	permissionMode := req.Env[envchain.PhaseEnvKey(profileName, "PERMISSION_MODE")]
	if permissionMode == "" && prof != nil {
		permissionMode = prof.PermissionMode
	}
	// Interactive policy follows the same profile-SSOT model: explicit
	// per-phase request env, then typed profile. Process env and the retired
	// global flag are intentionally excluded.
	interactivePolicy := req.Env[envchain.PhaseEnvKey(profileName, "INTERACTIVE_POLICY")]
	if interactivePolicy == "" && prof != nil {
		interactivePolicy = prof.InteractivePolicy
	}
	// Facet B: resolve the per-agent launch-time system prompt / rules
	// (profileName keys both the profile lookup and the EVOLVE_<AGENT>_* env).
	sysPrompt := systemprompt.Resolve(profileName, profileDir, req.Env)

	// WS-G1: dispatch through the chain via llmroute.Dispatch — the SAME
	// chain-walk implementation the advisor uses (cycle-435,
	// [[never_duplicate_centralize_via_design_patterns]]), rather than a
	// hand-rolled copy of it. Each attempt: build BridgeRequest for the
	// candidate CLI, Launch, normalize events. On a trigger exit (default
	// {80, 81, 124, 127} per cli_chain.go:defaultFallbackOnExit —
	// REPL-boot-timeout / artifact-timeout / coreutils-timeout /
	// missing-binary) Dispatch advances to the next candidate. Any other exit
	// (or success) stops the walk — a legitimate FAIL verdict from a model
	// never silently routes to a different CLI. Final attempt's (bres,
	// bridgeErr) is what the rest of the function consumes; events file
	// reflects the final CLI's stdout so cycleclassify sees what actually
	// happened last.
	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog []string
	llmroute.Dispatch(plan, func(candidateCLI string) (int, error) {
		i := len(attemptLog)
		if i > 0 {
			log.Diag().Infof(
				"[runner] phase=%s fallback %d/%d: trying cli=%s (previous=%s exit=%d)\n",
				phase, i+1, len(plan.Candidates), candidateCLI, plan.Candidates[i-1], bres.ExitCode)
		}
		bres, bridgeErr = b.bridge.Launch(ctx, core.BridgeRequest{
			CLI:                 candidateCLI,
			Profile:             profilePath,
			Model:               model,
			Prompt:              prompt,
			Workspace:           req.Workspace,
			Worktree:            req.Worktree,
			RunID:               req.RunID,
			ProjectRoot:         req.ProjectRoot,
			ArtifactPath:        artifactPath,
			Agent:               phase,
			Cycle:               req.Cycle,
			Env:                 req.Env,
			PermissionMode:      permissionMode,
			InteractivePolicy:   interactivePolicy,
			SystemPrompt:        sysPrompt,
			CorrectionDirective: req.CorrectionDirective,
			OperatorDirectives:  req.OperatorDirectives,
		})
		// Normalize per attempt so the final events file reflects the
		// final CLI's stdout — cycleclassify reads <phase>-events.ndjson
		// and we want it to describe what actually happened last.
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle); err != nil {
			log.Diag().Warnf("[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s=%d", candidateCLI, bres.ExitCode))
		// CLI-health bench: an exit-85 with a fresh benchable escalation
		// report (rate_limit class) is remembered ACROSS dispatches — run on
		// every candidate including the last, so the wall is recorded even
		// when no fallback remains (cycle-283). Staleness is judged against
		// the RUN start: the guard exists to exclude cross-PHASE leftovers in
		// the shared workspace, not earlier attempts of this same run.
		if bridgeErr != nil && bres.ExitCode == 85 {
			b.maybeBenchOnEscalation(req.ProjectRoot, req.Workspace, candidateCLI, start, req.Env)
		}
		return bres.ExitCode, bridgeErr
	})
	if len(attemptLog) > 1 {
		log.Diag().Infof("[runner] phase=%s dispatch chain: %s\n", phase, joinAttempts(attemptLog))
	}
	durationMS := b.nowFn().Sub(start).Milliseconds()

	// reconciled is set when a bridge ErrArtifactTimeout is overridden by a
	// well-formed deliverable on disk: control then FALLS THROUGH to the same
	// artifact-read + Classify path the happy case uses (so audit's EGPS gate
	// still applies — reconciliation can never ship a green-looking report whose
	// predicates are red). See the reconcile-on-timeout block below.
	reconciled := false
	if bridgeErr != nil {
		// A bridge artifact-wait timeout (exit 81) is a PROCESS failure, not a
		// verdict: the agent may have written its contracted deliverable just as
		// the bridge gave up on the wait window (the cycle-254/255 false-FAIL —
		// a complete PASS audit report recorded as FAIL). Reconcile against the
		// deliverable: if it is on disk and well-formed, trust its verdict (via
		// Classify) instead of synthesizing FAIL. Reconciliation can only UPGRADE
		// a timeout toward the agent's real verdict, never downgrade a real one.
		if errors.Is(bridgeErr, core.ErrArtifactTimeout) {
			roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree}
			if req.ProjectRoot != "" {
				// EvolveDir completes the roots (orchestrator-target
				// deliverables) AND locates the merged catalog for the
				// catalog-aware default.
				roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
			}
			res, verr := b.verifyFn(phase, roots)
			switch {
			case verr == nil && res.OK:
				// Deliverable survived the timeout — fall through to Classify.
				reconciled = true
			case b.optional:
				// Optional-phase soft-fail (Workstream D / cycle-120): no
				// trustworthy deliverable, but an optional phase's successor is
				// verdict-unconditional, so degrade to WARN and let the cycle
				// advance instead of aborting.
				msg := fmt.Sprintf("optional phase %q degraded: artifact never appeared (%v); cycle continues", phase, bridgeErr)
				if verr != nil {
					msg = fmt.Sprintf("%s [deliverable unverifiable: %v]", msg, verr)
				}
				return core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictWARN,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics: []core.Diagnostic{{
						Severity: "warning",
						Message:  msg,
					}},
				}, nil
			default:
				// Mandatory phase, no trustworthy deliverable (absent/malformed/
				// unverifiable): hard-fail as before, enriched with the
				// well-formedness violation when we have one.
				msg := bridgeErr.Error()
				if verr == nil && len(res.Violations) > 0 {
					msg = fmt.Sprintf("%s; deliverable not trustworthy: %s", msg, res.Violations[0].Message)
				}
				return core.PhaseResponse{
					Phase:        phase,
					Verdict:      core.VerdictFAIL,
					ArtifactsDir: req.Workspace,
					CostUSD:      bres.CostUSD,
					Tokens:       bres.Tokens,
					DurationMS:   durationMS,
					BootMS:       bres.BootMS,
					Diagnostics:  []core.Diagnostic{{Severity: "error", Message: msg}},
				}, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
			}
		} else {
			// Any non-timeout bridge error (launch/boot/safety/cost) means no
			// shippable work was produced — hard-fail, optional or not.
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictFAIL,
				ArtifactsDir: req.Workspace,
				CostUSD:      bres.CostUSD,
				Tokens:       bres.Tokens,
				DurationMS:   durationMS,
				BootMS:       bres.BootMS,
				Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
			}, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
		}
	}

	artifact := bres.Stdout
	// Prefer the on-disk deliverable over captured stdout whenever it exists and
	// verifies well-formed. bres.Stdout is bridge scrollback that can contain the
	// Deliverable Contract's own prompt-echoed example verdict sentinels — a real
	// PASS report was recorded as FAIL this way (cycle-603) when the agent wrote a
	// good deliverable then idled without a clean signal (a non-timeout completion,
	// so the reconcile-on-timeout fallback above never engaged). The agent's real
	// report is the file. This never downgrades: if the file is absent/malformed,
	// fall back to stdout exactly as before (fail-open preserved, no verdict gets
	// easier to game). On a reconciled timeout the file was already proven
	// well-formed above, so read it unconditionally.
	preferFile := reconciled || artifact == ""
	if !preferFile {
		roots := phasecontract.Roots{Workspace: req.Workspace, Worktree: req.Worktree}
		if req.ProjectRoot != "" {
			roots.EvolveDir = filepath.Join(req.ProjectRoot, ".evolve")
		}
		if res, verr := b.verifyFn(phase, roots); verr == nil && res.OK {
			preferFile = true
		}
	}
	if preferFile {
		if data, readErr := os.ReadFile(artifactPath); readErr == nil {
			artifact = string(data)
		}
	}

	// Best-effort: write the <phase>-stdout.clean.txt companion next to
	// the raw log. Default-on; set Options.DisableStdoutFilter=true to skip.
	// Filter failures NEVER block the phase — they WARN and continue,
	// because the raw log remains the forensic source of truth and
	// cyclecost / phaseobserver still read it directly.
	if !b.disableStdoutFilter {
		if err := b.stdoutFilter(req.Workspace, phase); err != nil {
			log.Diag().Warnf("[runner] WARN stdout filter phase=%s: %v\n", phase, err)
		}
	}

	verdict, diags, nextPhase := b.hooks.Classify(artifact, req, bres)

	resp := core.PhaseResponse{
		Phase:         phase,
		Verdict:       verdict,
		ArtifactsDir:  req.Workspace,
		NextPhase:     nextPhase,
		CostUSD:       bres.CostUSD,
		Tokens:        bres.Tokens,
		DurationMS:    durationMS,
		BootMS:        bres.BootMS,
		Diagnostics:   diags,
		ModelSource:   modelSource,
		ResolvedModel: model,
	}
	if reconciled {
		// A well-formed deliverable on a bridge timeout means the phase actually
		// COMPLETED — the timeout was a red herring (the bridge gave up on the
		// wait window just as, or after, the agent finished writing). So we treat
		// it exactly like a normal completed phase: nil error, the agent's own
		// Classify verdict authoritative. A reconciled FAIL therefore routes as a
		// real code-audit-fail (→ retro), NOT an infra-timeout retry — which is
		// both correct classification and avoids re-running a finished phase.
		// Reconciliation only ever upgrades a synthesized FAIL toward the agent's
		// real verdict; it never invents a PASS (Classify, incl. audit's EGPS
		// red_count gate, still decides).
		resp.Reconciled = true
		resp.Diagnostics = append(resp.Diagnostics, core.Diagnostic{
			Severity: "warning",
			Message:  fmt.Sprintf("bridge timed out (exit 81) but deliverable %s is well-formed; reconciled to %s from the agent's own report", artifactPath, verdict),
		})
		log.Diag().Infof("[runner] RECONCILED phase=%s exit=81 verdict=%s deliverable=%s\n", phase, verdict, artifactPath)
	}
	return resp, nil
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
	return b.String()
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

// ComposePrompt exposes the phase's prompt assembly (Hooks.ComposePrompt) as a
// public seam on BaseRunner so a caller can compose a prompt without launching
// the bridge — used by the cache-stable-prefix audit to inspect the static
// prefix for every BaseRunner-based phase. Run uses the same hook internally
// (see below); this method adds reach, not behavior.
func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}
