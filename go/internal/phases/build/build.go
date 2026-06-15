// Package build implements the GREEN code-writer phase. The phase
// boilerplate (profile lookup, prompt composition, bridge dispatch,
// artifact reading, response packaging) lives in internal/phases/runner;
// this file only encodes the build-specific variation points: agent name,
// artifact filename, prompt context, and classification rules.
//
// Verdict mapping (the artifact body classifier):
//
//   - "## Files Modified" missing or empty artifact → FAIL.
//   - All other GREEN paths → PASS.
package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// hooks implements runner.Hooks for the build phase.
type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseBuild) }
func (hooks) AgentPromptName() string                     { return "evolve-builder" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "build-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	if req.Workspace != "" && req.Env["EVOLVE_BUILD_PLANNER"] == "1" {
		// ADR-0050 Phase 3.7: prefer the envelope-served build-plan (populated at
		// the dispatch seam at advisory+); fall back to the original disk read at
		// off/shadow — byte-identical, including the empty-file case.
		if req.BuildPlan != "" {
			fmt.Fprintf(&b, "\n\n## Build Plan\n%s", req.BuildPlan)
		} else if data, err := os.ReadFile(filepath.Join(req.Workspace, "build-plan.md")); err == nil {
			fmt.Fprintf(&b, "\n\n## Build Plan\n%s", data)
		}
	}
	return b.String()
}

func (hooks) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	return classifyArtifact(artifact), nil, string(core.PhaseAudit)
}

// classifyArtifact derives the build verdict from report completeness; the
// accepted changed-files headings live in phasecontract.Build (single source).
func classifyArtifact(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	if phasecontract.Build.Complete(trimmed) {
		return core.VerdictPASS
	}
	return core.VerdictFAIL
}

// Config is the public constructor envelope. Preserved so callers
// outside the registry path (tests, programmatic embedding) keep
// working unchanged after the BaseRunner migration.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase wraps a core.PhaseRunner so callers still get a concrete
// *Phase return from New (preserves the public API).
type Phase struct{ core.PhaseRunner }

// New constructs the build phase from the BaseRunner.
func New(c Config) *Phase {
	base := runner.New(runner.Options{
		Hooks:   hooks{},
		Bridge:  c.Bridge,
		Prompts: c.Prompts,
		NowFn:   c.NowFn,
	})
	return &Phase{PhaseRunner: base}
}

// init registers the build phase factory at package load time.
func init() {
	registry.Register(string(core.PhaseBuild), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
