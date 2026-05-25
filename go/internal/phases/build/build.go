// Package build implements the GREEN code-writer phase. The phase
// boilerplate (profile lookup, prompt composition, bridge dispatch,
// artifact reading, response packaging) lives in
// internal/phases/runner; this file only encodes the build-specific
// variation points: agent name, artifact filename, prompt context,
// classification rules, and the cost-overrun guard.
//
// Cost guard:
//
//   - EVOLVE_BUILDER_COST_THRESHOLD (default 2.00 USD) is the soft cap.
//   - Cost > threshold + EVOLVE_BUILDER_COST_GUARD_STRICT=1 → FAIL.
//   - Cost > threshold + advisory (default) → PASS + diagnostic.
//
// Verdict mapping (the artifact body classifier):
//
//   - "## Files Modified" missing or empty artifact → FAIL.
//   - All other GREEN paths → PASS (possibly with cost diagnostic).
//
// The cost-overrun check will move out of this file into a shared
// CostGuardDecorator in Phase 2.5 commit 3 so other phases can opt in
// uniformly. For now it lives inline in Classify.
package build

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const defaultCostThresholdUSD = 2.00

// hooks implements runner.Hooks for the build phase.
type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseBuild) }
func (hooks) AgentPromptName() string                     { return "evolve-builder" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "build-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

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

func (hooks) Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict := classifyArtifact(artifact)
	var diags []core.Diagnostic

	threshold := parseFloatOrDefault(req.Env["EVOLVE_BUILDER_COST_THRESHOLD"], defaultCostThresholdUSD)
	if bres.CostUSD > threshold {
		msg := fmt.Sprintf("builder cost %.2f exceeded threshold %.2f", bres.CostUSD, threshold)
		severity := "warning"
		if req.Env["EVOLVE_BUILDER_COST_GUARD_STRICT"] == "1" {
			severity = "error"
			verdict = core.VerdictFAIL
		}
		diags = append(diags, core.Diagnostic{Severity: severity, Message: msg})
	}
	return verdict, diags, string(core.PhaseAudit)
}

func classifyArtifact(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	if !strings.Contains(trimmed, "## Files Modified") {
		return core.VerdictFAIL
	}
	return core.VerdictPASS
}

// parseFloatOrDefault returns d for empty or malformed input. Malformed
// values fall back silently to preserve the operator's ability to set
// EVOLVE_BUILDER_COST_THRESHOLD=auto without breaking the pipeline.
func parseFloatOrDefault(s string, d float64) float64 {
	if s == "" {
		return d
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return d
	}
	return v
}

// Config is the public constructor envelope. Preserved so callers
// outside the registry path (tests, programmatic embedding) keep
// working unchanged after the BaseRunner migration.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase wraps a runner.BaseRunner so callers still get a concrete
// *Phase return from New (preserves the public API).
type Phase struct{ *runner.BaseRunner }

// New constructs the build phase. The Hooks impl is internal; callers
// only know about Config.
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

// init registers the build phase factory at package load time. The
// dispatcher (cmd_phase.go, cmd_compose.go) looks up phases by name
// without an explicit switch — adding a phase is a new package + 1
// init() line, no edit to the dispatch table.
func init() {
	registry.Register(string(core.PhaseBuild), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
