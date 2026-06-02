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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
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
}

// BaseRunner is the Template Method implementation. Construct one per
// phase via New(); use it as a core.PhaseRunner.
type BaseRunner struct {
	hooks          Hooks
	bridge         core.Bridge
	prompts        *prompts.Loader
	nowFn          func() time.Time
	resolveLLM     func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	stdoutFilter   func(workspace, phase string) error
	eventsProducer func(workspace, phase, cli string, cycle int) error
	optional       bool
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
	return &BaseRunner{
		hooks:          opts.Hooks,
		bridge:         opts.Bridge,
		prompts:        opts.Prompts,
		nowFn:          nowFn,
		resolveLLM:     resolveLLM,
		stdoutFilter:   stdoutFilter,
		eventsProducer: eventsProducer,
		optional:       opts.Optional,
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

	// User-controlled policy pin (absolute): a pinned CLI/model for this phase
	// overrides env/profile/default resolution. Keyed by phase name. Validated
	// against the profile guardrails (allowed_clis + model_tier_envelope) — an
	// out-of-bounds pin hard-fails the phase loudly rather than silently
	// breaching the trust-kernel constraints. Escape hatch: EVOLVE_POLICY_BYPASS=1.
	var pin *policy.Pin
	if !envchain.Bool("EVOLVE_POLICY_BYPASS", req.Env, false) {
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
			fmt.Fprintf(os.Stderr, "[runner] phase=%s policy pin: cli=%q model=%q\n", phase, p.CLI, p.Model)
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
			fmt.Fprintf(os.Stderr, "[runner] phase=%s capability probe reordered chain: %v -> %v\n",
				phase, preCandidates, plan.Candidates)
		}
	}
	cli := plan.Candidates[0]
	// Disambiguating dispatch log: tells observers which CLI is actually being
	// invoked and why (an output stream saying `model: claude-sonnet-4-6` could
	// otherwise be misread as "codex delegating to claude").
	if len(plan.Candidates) > 1 {
		fmt.Fprintf(os.Stderr, "[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s fallback=%v triggers=%v\n",
			phase, profileName, cli, plan.PrimarySource, profilePath, plan.Candidates[1:], plan.Triggers)
	} else {
		fmt.Fprintf(os.Stderr, "[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
			phase, profileName, cli, plan.PrimarySource, profilePath)
	}
	model := plan.Model
	// Per-phase permission-mode override: EVOLVE_<AGENT>_PERMISSION_MODE,
	// resolved here with the AGENT name (profileName) so the env key matches
	// CLAUDE.md's convention (EVOLVE_TDD_ENGINEER_PERMISSION_MODE, not
	// EVOLVE_TDD_*). Passed as typed config — the bridge realizes it per-CLI
	// via the LaunchIntent (no raw --permission-mode leak into non-claude
	// launch commands). Empty = profile/realizer default (bypass).
	permissionMode := envchain.Resolve(envchain.PhaseEnvKey(profileName, "PERMISSION_MODE"), req.Env, "", "")
	// Facet B: resolve the per-agent launch-time system prompt / rules
	// (profileName keys both the profile lookup and the EVOLVE_<AGENT>_* env).
	sysPrompt := systemprompt.Resolve(profileName, profileDir, req.Env)

	// WS-G1: dispatch through the chain. Each attempt: build BridgeRequest
	// for the candidate CLI, Launch, normalize events. On a trigger exit
	// (default {80, 81, 124, 127} per cli_chain.go:defaultFallbackOnExit
	// — REPL-boot-timeout / artifact-timeout / coreutils-timeout /
	// missing-binary) we advance to the next candidate. Any other exit
	// (or success) breaks the loop — a legitimate FAIL verdict from a
	// model never silently routes to a different CLI. Final attempt's
	// (bres, bridgeErr, cli) is what the rest of the function consumes;
	// events file reflects the final CLI's stdout so cycleclassify sees
	// what actually happened last.
	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog []string
	for i, candidateCLI := range plan.Candidates {
		if i > 0 {
			fmt.Fprintf(os.Stderr,
				"[runner] phase=%s fallback %d/%d: trying cli=%s (previous=%s exit=%d)\n",
				phase, i+1, len(plan.Candidates), candidateCLI, plan.Candidates[i-1], bres.ExitCode)
		}
		bres, bridgeErr = b.bridge.Launch(ctx, core.BridgeRequest{
			CLI:            candidateCLI,
			Profile:        profilePath,
			Model:          model,
			Prompt:         prompt,
			Workspace:      req.Workspace,
			Worktree:       req.Worktree,
			ProjectRoot:    req.ProjectRoot,
			ArtifactPath:   artifactPath,
			Agent:          phase,
			Cycle:          req.Cycle,
			Env:            req.Env,
			PermissionMode: permissionMode,
			SystemPrompt:   sysPrompt,
		})
		// Normalize per attempt so the final events file reflects the
		// final CLI's stdout — cycleclassify reads <phase>-events.ndjson
		// and we want it to describe what actually happened last.
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle); err != nil {
			fmt.Fprintf(os.Stderr, "[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s=%d", candidateCLI, bres.ExitCode))
		// (Pre-existing WS-G dead assignment removed in cycle-122 Fix 2:
		// `cli` was assigned here but never read downstream — the
		// per-attempt CLI lives in attemptLog instead. Comment kept so
		// `git blame` reveals the cleanup intent for a future reader.)
		if bridgeErr == nil {
			break // success
		}
		if !plan.TriggersFallback(bres.ExitCode) {
			break // non-trigger error: surface to caller (real FAIL, real timeout on mandatory phase, etc.)
		}
		// trigger exit + more candidates → loop continues
	}
	if len(attemptLog) > 1 {
		fmt.Fprintf(os.Stderr, "[runner] phase=%s dispatch chain: %s\n", phase, joinAttempts(attemptLog))
	}
	durationMS := b.nowFn().Sub(start).Milliseconds()

	if bridgeErr != nil {
		// Optional-phase soft-fail (Workstream D): an OPTIONAL phase whose
		// artifact never appeared (ErrArtifactTimeout) degrades to WARN with a
		// nil error so the orchestrator advances instead of aborting the whole
		// cycle. Safe because an optional phase's state-machine successor is
		// verdict-unconditional (build-planner→build). Any OTHER bridge error,
		// or a timeout on a MANDATORY phase, still hard-fails as before. This
		// is the cycle-120 fix: an advisory build-planner timeout must not kill
		// the cycle.
		if b.optional && errors.Is(bridgeErr, core.ErrArtifactTimeout) {
			return core.PhaseResponse{
				Phase:        phase,
				Verdict:      core.VerdictWARN,
				ArtifactsDir: req.Workspace,
				CostUSD:      bres.CostUSD,
				Tokens:       bres.Tokens,
				DurationMS:   durationMS,
				Diagnostics: []core.Diagnostic{{
					Severity: "warning",
					Message:  fmt.Sprintf("optional phase %q degraded: artifact never appeared (%v); cycle continues", phase, bridgeErr),
				}},
			}, nil
		}
		return core.PhaseResponse{
			Phase:        phase,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			CostUSD:      bres.CostUSD,
			Tokens:       bres.Tokens,
			DurationMS:   durationMS,
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
		}, fmt.Errorf("%s: bridge: %w", phase, bridgeErr)
	}

	artifact := bres.Stdout
	if artifact == "" {
		if data, readErr := os.ReadFile(artifactPath); readErr == nil {
			artifact = string(data)
		}
	}

	// Best-effort: write the <phase>-stdout.clean.txt companion next to
	// the raw log. Default-on; set EVOLVE_STDOUT_FILTER=off to skip.
	// Filter failures NEVER block the phase — they WARN and continue,
	// because the raw log remains the forensic source of truth and
	// cyclecost / phaseobserver still read it directly.
	if envchain.Resolve("EVOLVE_STDOUT_FILTER", req.Env, "", "on") != "off" {
		if err := b.stdoutFilter(req.Workspace, phase); err != nil {
			fmt.Fprintf(os.Stderr, "[runner] WARN stdout filter phase=%s: %v\n", phase, err)
		}
	}

	verdict, diags, nextPhase := b.hooks.Classify(artifact, req, bres)

	return core.PhaseResponse{
		Phase:        phase,
		Verdict:      verdict,
		ArtifactsDir: req.Workspace,
		NextPhase:    nextPhase,
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
		Diagnostics:  diags,
	}, nil
}
