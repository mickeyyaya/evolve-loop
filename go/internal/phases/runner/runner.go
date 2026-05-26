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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
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

	agent, err := b.prompts.Agent(b.hooks.AgentPromptName())
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("%s: load agent: %w", phase, err)
	}

	prompt := b.hooks.ComposePrompt(agent.Body, req)
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
	profileCLI := ""
	profileModelTier := ""
	if loader := profiles.NewFromDir(profileDir); loader != nil {
		if prof, err := loader.Get(profileName); err == nil {
			profileCLI = prof.CLI
			profileModelTier = prof.ModelTierDefault
		}
	}
	cli := envchain.Resolve("EVOLVE_CLI", req.Env, profileCLI, "claude-tmux")
	cliSource := "default"
	switch {
	case req.Env["EVOLVE_CLI"] != "" || os.Getenv("EVOLVE_CLI") != "":
		cliSource = "env(EVOLVE_CLI)"
	case profileCLI != "":
		cliSource = "profile." + profileName + ".cli"
	}
	// Disambiguating dispatch log: tells observers which CLI is actually
	// being invoked and why. Without this an output stream that says
	// `model: claude-sonnet-4-6` could be misread as "codex delegating
	// to claude" when the actual cause is "runner ignored profile.cli".
	fmt.Fprintf(os.Stderr, "[runner] phase=%s agent=%s cli=%s (source=%s) profile=%s\n",
		phase, profileName, cli, cliSource, profilePath)

	modelKey := envchain.PhaseEnvKey(phase, "MODEL")
	model := envchain.Resolve(modelKey, req.Env, profileModelTier, b.hooks.DefaultModel())
	// Resolve the "auto" sentinel through llm_config.json + profile chain.
	// Hooks.DefaultModel returns "auto" for most phases as a signal that
	// the resolver should pick a concrete model based on phase role +
	// llm_config tier mapping. claude -p rejects "auto" with HTTP 404
	// ("There's an issue with the selected model (auto). It may not exist
	// or you may not have access to it."), so the resolution MUST happen
	// before bridge dispatch. Source: cycle 106 (2026-05-25) integration
	// smoke that uncovered the v12.1.0 missing wire.
	if model == "auto" {
		if res, err := b.resolveLLM(phase, resolvellm.Options{}); err == nil {
			switch {
			case res.Model != "":
				model = res.Model
			case res.ModelTier != "":
				model = res.ModelTier
			}
		}
	}
	// Per-phase permission-mode override: EVOLVE_<AGENT>_PERMISSION_MODE,
	// resolved here with the AGENT name (profileName) so the env key matches
	// CLAUDE.md's convention (EVOLVE_TDD_ENGINEER_PERMISSION_MODE, not
	// EVOLVE_TDD_*). Passed as typed config — the bridge realizes it per-CLI
	// via the LaunchIntent (no raw --permission-mode leak into non-claude
	// launch commands). Empty = profile/realizer default (bypass).
	permissionMode := envchain.Resolve(envchain.PhaseEnvKey(profileName, "PERMISSION_MODE"), req.Env, "", "")

	bres, bridgeErr := b.bridge.Launch(ctx, core.BridgeRequest{
		CLI:            cli,
		Profile:        profilePath,
		Model:          model,
		Prompt:         prompt,
		Workspace:      req.Workspace,
		Worktree:       req.Worktree,
		ArtifactPath:   artifactPath,
		Agent:          phase,
		Cycle:          req.Cycle,
		Env:            req.Env,
		PermissionMode: permissionMode,
	})
	durationMS := b.nowFn().Sub(start).Milliseconds()

	// Always-on: normalize the raw logs into <phase>-events.ndjson (ADR-0020),
	// the unified stream cyclecost (cost) + cycleclassify (infra) read. Runs
	// BEFORE the bridge-error guard on purpose: a phase that fails on a
	// timeout / 429 / 529 is exactly the infrastructure failure cycleclassify
	// must detect, and the bridge writes the raw logs even when Launch errors.
	// A failure WARNs loudly rather than blocking — the raw log remains the
	// forensic source of truth and can be re-normalized — but it degrades cost
	// accounting + failure classification, so it must never be silent.
	if err := b.eventsProducer(req.Workspace, phase, cli, req.Cycle); err != nil {
		fmt.Fprintf(os.Stderr, "[runner] WARN events producer phase=%s: %v (cost/classification degraded)\n", phase, err)
	}

	if bridgeErr != nil {
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
