// Package scout implements the discovery and planning phase as a
// core.PhaseRunner. It composes a scout prompt with the cycle context,
// dispatches the LLM via the bridge, and classifies the resulting
// scout-report.md into a verdict.
//
// Verdict mapping (artifact body inspected, all checks case-sensitive):
//
//   - convergence-confirmation strategy + no Proposed Tasks → SKIPPED
//   - "## Proposed Tasks" section with at least one item → PASS
//   - empty/missing artifact, or "## Proposed Tasks" missing → FAIL
//
// Convergence is the only path that maps to SKIPPED. Empty backlog
// elsewhere is a real failure (Scout has nothing to feed Triage).
package scout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseflags"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const phaseName = string(core.PhaseScout)

// Config is the constructor envelope.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase implements core.PhaseRunner for the scout discovery stage.
type Phase struct {
	bridge  core.Bridge
	prompts *prompts.Loader
	nowFn   func() time.Time
}

// New builds a Phase.
func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{bridge: c.Bridge, prompts: c.Prompts, nowFn: nowFn}
}

// Name returns the phase identity string ("scout").
func (p *Phase) Name() string { return phaseName }

// Run executes the scout discovery cycle.
func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("scout: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("scout: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-scout")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("scout: load agent: %w", err)
	}

	prompt := composePrompt(agent.Body, req)
	artifactPath := filepath.Join(req.Workspace, "scout-report.md")
	profileDir := filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	profilePath := filepath.Join(profileDir, "scout.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Env["EVOLVE_SCOUT_MODEL"]
	if model == "" {
		model = "auto"
	}

	extraFlags := phaseflags.For("scout").Resolve(profileDir, req.Env)

	bres, bridgeErr := p.bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       prompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        "scout",
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
		}, fmt.Errorf("scout: bridge: %w", bridgeErr)
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}

	strategy := req.Context["strategy"]
	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      classify(content, strategy),
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseTriage),
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
	if s := req.Context["strategy"]; s != "" {
		fmt.Fprintf(&b, "- strategy: %s\n", s)
	}
	return b.String()
}

// proposedTasksRE matches at least one Markdown list item (numbered or
// bulleted) under "## Proposed Tasks". Used to confirm Scout produced
// a non-empty backlog.
var proposedTasksRE = regexp.MustCompile(`(?m)^## Proposed Tasks\b[\s\S]*?^[*\-0-9]+\.?\s+\S`)

func classify(content, strategy string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	hasBacklog := proposedTasksRE.MatchString(trimmed)
	if strategy == "convergence-confirmation" && !hasBacklog {
		return core.VerdictSKIPPED
	}
	if !hasBacklog {
		return core.VerdictFAIL
	}
	return core.VerdictPASS
}
