// Package intent implements the pre-Scout intent capture phase as a
// core.PhaseRunner. Its job is to drive the LLM (via the bridge) to
// produce a structured intent.md, then translate the artifact into a
// PhaseResponse verdict.
//
// Semantics (plan §intent, CLAUDE.md env-var table):
//
//   - EVOLVE_INTENT_DELTA=1 → emit intent-delta.md or "[intent-unchanged]"
//     stub; the latter maps to a SKIPPED verdict so the cycle short-circuits
//     intent and re-uses the prior batch's intent.
//   - Required artifact sections (full mode): YAML frontmatter with at
//     least goal: and acceptance_checks:. Missing either → FAIL.
//
// The phase package owns only prompt composition + artifact-to-verdict
// classification. Bridge subprocess + filesystem semantics live in
// internal/adapters/bridge; agent-file loading lives in internal/prompts.
package intent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const phaseName = string(core.PhaseIntent)

// Config is the constructor envelope. NowFn is the wall-clock seam
// (defaults to time.Now); the others are required.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase implements core.PhaseRunner for the intent capture stage.
type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
	nowFn   func() time.Time
}

// New constructs a Phase from the given Config. The constructor itself
// does not validate the dependencies — Run() reports any missing
// dependencies at call time so the same struct can be used for both
// happy-path and error-path tests.
func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{bridge: c.Bridge, prompts: c.Prompts, nowFn: nowFn}
}

// Name returns the phase identity string ("intent").
func (p *Phase) Name() string { return phaseName }

// Run executes the intent capture cycle.
func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("intent: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("intent: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-intent")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("intent: load agent: %w", err)
	}

	delta := req.Env["EVOLVE_INTENT_DELTA"] == "1"
	prompt := composePrompt(agent.Body, req, delta)

	artifactName := "intent.md"
	if delta {
		artifactName = "intent-delta.md"
	}
	artifactPath := filepath.Join(req.Workspace, artifactName)
	profilePath := filepath.Join(req.ProjectRoot, ".evolve", "profiles", "intent.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Env["EVOLVE_INTENT_MODEL"]
	if model == "" {
		model = "auto"
	}

	bres, bridgeErr := p.bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       prompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        "intent",
		Cycle:        req.Cycle,
		Env:          req.Env,
	})
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if bridgeErr != nil {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			CostUSD:      bres.CostUSD,
			Tokens:       bres.Tokens,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: bridgeErr.Error()},
			},
		}, fmt.Errorf("intent: bridge: %w", bridgeErr)
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      classify(content, delta),
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseScout),
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
	}, nil
}

// composePrompt builds the agent prompt body plus a cycle-context block
// the agent uses to ground its output.
func composePrompt(body string, req core.PhaseRequest, delta bool) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if delta {
		b.WriteString("- mode: delta (emit intent-delta.md or [intent-unchanged] stub if goal_hash matches state.json:currentBatch.goalHash)\n")
	} else {
		b.WriteString("- mode: full\n")
	}
	return b.String()
}

// classify maps an intent.md artifact body to a verdict.
func classify(content string, delta bool) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	if delta && strings.Contains(trimmed, "[intent-unchanged]") {
		return core.VerdictSKIPPED
	}
	if strings.Contains(trimmed, "goal:") && strings.Contains(trimmed, "acceptance_checks:") {
		return core.VerdictPASS
	}
	return core.VerdictFAIL
}
