// Package triage implements the cycle-scope triage phase as a
// core.PhaseRunner. Triage sits between Scout and Plan-review:
// it reads the Scout backlog plus carryoverTodos and decides top_n[]
// for THIS cycle, deferred[] for next, and dropped[].
//
// Verdict mapping:
//
//   - EVOLVE_TRIAGE_DISABLE=1 → SKIPPED (bridge never called)
//   - "## top_n" with at least one item → PASS
//   - empty top_n or empty artifact → FAIL
package triage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const phaseName = string(core.PhaseTriage)

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
	if req.Env["EVOLVE_TRIAGE_DISABLE"] == "1" {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictSKIPPED,
			NextPhase:    string(core.PhaseTDD),
			ArtifactsDir: req.Workspace,
			DurationMS:   p.nowFn().Sub(start).Milliseconds(),
		}, nil
	}
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("triage: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("triage: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-triage")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("triage: load agent: %w", err)
	}

	prompt := composePrompt(agent.Body, req)
	artifactPath := filepath.Join(req.Workspace, "triage-report.md")
	profilePath := filepath.Join(req.ProjectRoot, ".evolve", "profiles", "triage.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Env["EVOLVE_TRIAGE_MODEL"]
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
		Agent:        "triage",
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
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
		}, fmt.Errorf("triage: bridge: %w", bridgeErr)
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
		NextPhase:    string(core.PhaseTDD),
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
	if s := req.Context["carryover_summary"]; s != "" {
		fmt.Fprintf(&b, "- carryover_summary: %s\n", s)
	}
	return b.String()
}

// topNHeadingRE locates the "## top_n" section heading.
var topNHeadingRE = regexp.MustCompile(`(?m)^## top_n\b`)

// listItemRE matches a single non-empty Markdown list item line.
var listItemRE = regexp.MustCompile(`(?m)^[-*]\s+\S`)

// nextHeadingRE finds the next "## " section heading.
var nextHeadingRE = regexp.MustCompile(`(?m)^## `)

func classify(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	loc := topNHeadingRE.FindStringIndex(trimmed)
	if loc == nil {
		return core.VerdictFAIL
	}
	// Slice from end of "## top_n" line to the next "## " heading
	// (or end-of-string). RE2 has no lookahead, so do it by hand.
	body := trimmed[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	if !listItemRE.MatchString(body) {
		return core.VerdictFAIL
	}
	return core.VerdictPASS
}
