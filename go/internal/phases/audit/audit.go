// Package audit implements the EGPS gate phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// audit-specific variation points.
//
// Audit is the EGPS gate: PASS requires BOTH a parseable PASS verdict
// in audit-report.md AND red_count == 0 in acs-verdict.json.
// EVOLVE_STRICT_AUDIT=1 additionally promotes WARN to FAIL.
//
// Verdict mapping:
//   - empty artifact / missing "## Verdict" heading → FAIL
//   - acs-verdict.json missing or unparseable → FAIL + error diag
//   - acs-verdict.json red_count > 0 → FAIL + EGPS diag
//   - WARN + EVOLVE_STRICT_AUDIT=1 → FAIL
//   - otherwise → whatever the audit-report.md heading says (PASS/WARN/FAIL/SKIPPED)
//
// Default model is "opus" for adversarial cross-family diversity from
// the build phase's Sonnet.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// verdictRE captures the verdict word from the canonical heading shape
// "## Verdict\n**PASS**" (case-sensitive, allows surrounding whitespace).
var verdictRE = regexp.MustCompile(`(?m)^##\s*Verdict\s*\n\*\*(PASS|FAIL|WARN|SKIPPED)\*\*`)

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string                     { return "evolve-auditor" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "audit-report.md" }
func (hooks) DefaultModel() string                        { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
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

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict := extractAuditVerdict(artifact)
	var diags []core.Diagnostic

	verdictPath := filepath.Join(req.Workspace, "acs-verdict.json")
	redCount, acsErr := readRedCount(verdictPath)
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
	return verdict, diags, string(core.PhaseShip)
}

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

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseAudit), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
