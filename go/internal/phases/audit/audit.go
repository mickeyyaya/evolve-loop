// Package audit implements the EGPS-gate phase as a core.PhaseRunner.
// Audit is the only phase that fuses two artifacts into one verdict:
//
//  1. audit-report.md — the LLM auditor's narrative verdict
//     (heading shape "## Verdict\n**PASS**" per project convention).
//  2. acs-verdict.json — the EGPS predicate runner's red_count.
//
// Per CLAUDE.md env-var table (v10.0.0+): the cycle ships only if
// audit_verdict ∈ {PASS, WARN} AND red_count == 0. EVOLVE_STRICT_AUDIT=1
// promotes WARN → FAIL.
//
// Adversarial-audit framing (positive evidence required) lives in the
// agent prompt; the package only enforces verdict + EGPS arithmetic.
package audit

import (
	"context"
	"encoding/json"
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

const phaseName = string(core.PhaseAudit)

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
	if p.bridge == nil {
		return core.PhaseResponse{}, fmt.Errorf("audit: bridge required")
	}
	if p.prompts == nil {
		return core.PhaseResponse{}, fmt.Errorf("audit: prompts loader required")
	}

	agent, err := p.prompts.Agent("evolve-auditor")
	if err != nil {
		return core.PhaseResponse{}, fmt.Errorf("audit: load agent: %w", err)
	}

	prompt := composePrompt(agent.Body, req)
	artifactPath := filepath.Join(req.Workspace, "audit-report.md")
	verdictPath := filepath.Join(req.Workspace, "acs-verdict.json")
	profileDir := filepath.Join(req.ProjectRoot, ".evolve", "profiles")
	profilePath := filepath.Join(profileDir, "audit.json")

	cli := req.Env["EVOLVE_CLI"]
	if cli == "" {
		cli = "claude-p"
	}
	model := req.Env["EVOLVE_AUDIT_MODEL"]
	if model == "" {
		model = "opus" // Adversarial-audit default: different family from Builder.
	}

	extraFlags := phaseflags.For("audit").Resolve(profileDir, req.Env)

	bres, bridgeErr := p.bridge.Launch(ctx, core.BridgeRequest{
		CLI:          cli,
		Profile:      profilePath,
		Model:        model,
		Prompt:       prompt,
		Workspace:    req.Workspace,
		Worktree:     req.Worktree,
		ArtifactPath: artifactPath,
		Agent:        "audit",
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
		}, fmt.Errorf("audit: bridge: %w", bridgeErr)
	}

	content := bres.Stdout
	if content == "" {
		if b, err := os.ReadFile(artifactPath); err == nil {
			content = string(b)
		}
	}

	auditVerdict := extractAuditVerdict(content)
	redCount, acsErr := readRedCount(verdictPath)

	var diags []core.Diagnostic
	verdict := auditVerdict

	if acsErr != nil {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("acs-verdict.json: %s", acsErr.Error()),
		})
		verdict = core.VerdictFAIL
	} else if redCount > 0 {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  fmt.Sprintf("EGPS: red_count=%d (cycle ships only when red_count==0)", redCount),
		})
		verdict = core.VerdictFAIL
	}

	if verdict == core.VerdictWARN && req.Env["EVOLVE_STRICT_AUDIT"] == "1" {
		verdict = core.VerdictFAIL
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "EVOLVE_STRICT_AUDIT=1 promoted WARN to FAIL",
		})
	}

	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      verdict,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseShip),
		CostUSD:      bres.CostUSD,
		Tokens:       bres.Tokens,
		DurationMS:   durationMS,
		Diagnostics:  diags,
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

// verdictRE captures the verdict word from the canonical heading shape
// "## Verdict\n**PASS**" (case-sensitive, allows surrounding whitespace).
var verdictRE = regexp.MustCompile(`(?m)^##\s*Verdict\s*\n\*\*(PASS|FAIL|WARN|SKIPPED)\*\*`)

func extractAuditVerdict(content string) string {
	m := verdictRE.FindStringSubmatch(content)
	if m == nil {
		return core.VerdictFAIL
	}
	return m[1]
}

func readRedCount(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	var v struct {
		RedCount int `json:"red_count"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	return v.RedCount, nil
}
