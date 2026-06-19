// Package retro implements the FAIL/WARN-only post-mortem phase as a
// core.PhaseRunner. It runs only when the previous verdict is FAIL or
// WARN; PASS cycles short-circuit to SKIPPED (the Memo phase handles
// PASS-cycle observation).
//
// Verdict mapping:
//
//   - previous verdict != FAIL/WARN → SKIPPED, no bridge call
//   - retrospective.md non-empty AND at least one failure-lesson*.yaml
//     present in workspace → PASS
//   - otherwise → FAIL
package retro

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

const phaseName = string(core.PhaseRetro)

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

	prev := req.Context["previous_verdict"]
	if prev != core.VerdictFAIL && prev != core.VerdictWARN {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictSKIPPED,
			NextPhase:    string(core.PhaseEnd),
			ArtifactsDir: req.Workspace,
			DurationMS:   p.nowFn().Sub(start).Milliseconds(),
		}, nil
	}
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-retrospective")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("retro: load agent: %w", err)
	}

	prompt := composePrompt(agent.Body, req, prev)
	artifactPath := filepath.Join(req.Workspace, "retrospective-report.md")
	profilePath := filepath.Join(req.ProjectRoot, ".evolve", "profiles", "retrospective.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-tmux"
	}
	model := req.Env["EVOLVE_RETRO_MODEL"]
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
		Agent:        "retrospective",
		Cycle:        req.Cycle,
		Env:          req.Env,
	})
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if bridgeErr != nil {
		// GAP 9 (self-healing): retro is the failure-analysis phase on the
		// audit-FAIL path. A non-nil error from Run propagates to RunCycle as a
		// hard abort that stops the WHOLE batch (the runs 154-162 abort mode) — a
		// failure in the failure-handler must never be fatal. Return a FAIL verdict
		// with NIL error so the orchestrator routes through decideAfterRetro
		// (failure-adapter: retry/block/proceed) instead of aborting. The bridge
		// error is preserved as a diagnostic for forensics; NextPhase is advisory
		// (decideAfterRetro picks the real successor from the verdict + history).
		fmt.Fprintf(os.Stderr, "[retro] WARN bridge failed (%v) — emitting FAIL verdict; orchestrator routes via failure-adapter (non-fatal)\n", bridgeErr)
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			NextPhase:    string(core.PhaseEnd),
			CostUSD:      bres.CostUSD,
			Tokens:       bres.Tokens,
			DurationMS:   durationMS,
			Diagnostics:  []core.Diagnostic{{Severity: "error", Message: bridgeErr.Error()}},
		}, nil
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}
	verdict := core.VerdictPASS
	if strings.TrimSpace(content) == "" || !hasFailureLesson(req.Workspace) {
		verdict = core.VerdictFAIL
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      verdict,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseEnd),
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
	}, nil
}

func composePrompt(body string, req core.PhaseRequest, prev string) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- previous_verdict: %s\n", prev)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	return b.String()
}

// hasFailureLesson reports whether the workspace contains any file
// matching failure-lesson*.yaml. The retrospective agent emits the file
// alongside retrospective.md; without it, the retro is incomplete.
func hasFailureLesson(ws string) bool {
	entries, err := os.ReadDir(ws)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, "failure-lesson") && strings.HasSuffix(n, ".yaml") {
			return true
		}
	}
	return false
}
