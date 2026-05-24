// Package tdd implements the test-first phase as a core.PhaseRunner.
// TDD writes failing tests BEFORE Builder writes any production code.
// The RED phase is the proof of understanding.
//
// Verdict mapping:
//
//   - EVOLVE_TEST_PHASE_ENABLED=0 → SKIPPED (Builder writes own predicates)
//   - team-context.md present with "## Acceptance" + "## RED Tests" → PASS
//   - missing required sections, empty, or bridge error → FAIL
package tdd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseflags"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const phaseName = string(core.PhaseTDD)

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
	nowFn   func() time.Time
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{bridge: c.Bridge, prompts: c.Prompts, nowFn: nowFn}
}

func (p *Phase) Name() string { return phaseName }

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if req.Env["EVOLVE_TEST_PHASE_ENABLED"] == "0" {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictSKIPPED,
			NextPhase:    string(core.PhaseBuild),
			ArtifactsDir: req.Workspace,
			DurationMS:   p.nowFn().Sub(start).Milliseconds(),
		}, nil
	}
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("tdd: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("tdd: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-tdd-engineer")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("tdd: load agent: %w", err)
	}

	prompt := composePrompt(agent.Body, req)
	artifactPath := filepath.Join(req.Workspace, "team-context.md")
	profileDir := filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	profilePath := filepath.Join(profileDir, "tdd.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Env["EVOLVE_TDD_MODEL"]
	if model == "" {
		model = "auto"
	}

	extraFlags := phaseflags.For("tdd").Resolve(profileDir, req.Env)

	bres, bridgeErr := p.bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       prompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        "tdd",
		Cycle:        req.Cycle,
		Env:          req.Env,
		ExtraFlags:   extraFlags,
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
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
		}, fmt.Errorf("tdd: bridge: %w", bridgeErr)
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      classify(content),
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseBuild),
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
	}, nil
}

func composePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	return b.String()
}

func classify(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	if !strings.Contains(trimmed, "## Acceptance") {
		return core.VerdictFAIL
	}
	if !strings.Contains(trimmed, "## RED Tests") {
		return core.VerdictFAIL
	}
	return core.VerdictPASS
}
