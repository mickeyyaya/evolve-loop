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
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseflags"
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
}

// BaseRunner is the Template Method implementation. Construct one per
// phase via New(); use it as a core.PhaseRunner.
type BaseRunner struct {
	hooks   Hooks
	bridge  core.Bridge
	prompts *prompts.Loader
	nowFn   func() time.Time
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
	return &BaseRunner{
		hooks:   opts.Hooks,
		bridge:  opts.Bridge,
		prompts: opts.Prompts,
		nowFn:   nowFn,
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

	cli := envchain.Resolve("EVOLVE_CLI", req.Env, "", "claude-p")
	modelKey := envchain.PhaseEnvKey(phase, "MODEL")
	model := envchain.Resolve(modelKey, req.Env, "", b.hooks.DefaultModel())
	// Resolve the "auto" sentinel through llm_config.json + profile chain.
	// Hooks.DefaultModel returns "auto" for most phases as a signal that
	// the resolver should pick a concrete model based on phase role +
	// llm_config tier mapping. claude -p rejects "auto" with HTTP 404
	// ("There's an issue with the selected model (auto). It may not exist
	// or you may not have access to it."), so the resolution MUST happen
	// before bridge dispatch. Source: cycle 106 (2026-05-25) integration
	// smoke that uncovered the v12.1.0 missing wire.
	if model == "auto" {
		if res, err := resolvellm.Resolve(phase, resolvellm.Options{}); err == nil {
			switch {
			case res.Model != "":
				model = res.Model
			case res.ModelTier != "":
				model = res.ModelTier
			}
		}
	}
	// phaseflags.For takes the AGENT name (profileName) so that:
	// (a) profile JSON lookup matches the file on disk, and
	// (b) per-phase env-var keys match CLAUDE.md's documented
	//     EVOLVE_<AGENT_UPPER>_<KEY> convention (e.g.,
	//     EVOLVE_TDD_ENGINEER_PERMISSION_MODE, not EVOLVE_TDD_*).
	extraFlags := phaseflags.For(profileName).Resolve(profileDir, req.Env)

	bres, bridgeErr := b.bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       prompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        phase,
		Cycle:        req.Cycle,
		Env:          req.Env,
		ExtraFlags:   extraFlags,
	})
	durationMS := b.nowFn().Sub(start).Milliseconds()

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
