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
	"strings"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolveloop/go/internal/prompts"
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
	if req.BuildPlan != "" {
		fmt.Fprintf(&b, "\n\n## Build Plan\n%s", req.BuildPlan)
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
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
	// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
	PhaseIO config.Stage
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
		PhaseIO: c.PhaseIO,
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
