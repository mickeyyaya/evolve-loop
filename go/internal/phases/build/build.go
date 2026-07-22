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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
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
	// Cycle-776 (fleet-lane-provisioning-split residual): render the pinned
	// lane scope so the builder binds ONLY to this lane's task ids.
	if scope := runner.LaneScope(req); scope != "" {
		fmt.Fprintf(&b, "- fleet_scope: this cycle is one fleet lane; bind ONLY to tasks whose id is in this assigned set, ignore all others: %s\n", scope)
	}
	if req.BuildPlan != "" {
		fmt.Fprintf(&b, "\n\n## Build Plan\n%s", req.BuildPlan)
	}
	// ADR-0076 slice C: an adopted continuation resumes a prior attempt's
	// preserved work — hand the builder that attempt's failure findings so it
	// finishes what remains instead of rediscovering it. Absent key ⇒
	// byte-identical legacy prompt.
	if findings := req.Context["continuation_findings"]; findings != "" {
		fmt.Fprintf(&b, "\n\n## Prior Attempt Findings\nThis worktree RESUMES a prior attempt's preserved work — do not restart or discard it. The prior attempt failed with the findings below; resume, complete the remaining gaps they describe, and re-verify the whole change.\n\n%s", findings)
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
	// CompactPrompts strips the on-demand reference tail from the disk-loaded agent
	// doc before dispatch. Value flows from workflow.compact_prompts (policy.json);
	// never set to a bare literal here (standing rule: phase-settings-from-config).
	CompactPrompts bool
}

// Phase is the build phase, a runner.BaseRunner specialized with the
// build-specific hooks. It embeds *runner.BaseRunner concretely (like every
// other BaseRunner-based phase) so BaseRunner's public seams — including
// ComposePrompt, which the cache-stable-prefix audit relies on — promote onto
// *Phase. (Previously it embedded the core.PhaseRunner interface, which hid
// those seams behind the two-method contract.)
type Phase struct{ *runner.BaseRunner }

// New constructs the build phase from the BaseRunner.
func New(c Config) *Phase {
	base := runner.New(runner.Options{
		Hooks:          hooks{},
		Bridge:         c.Bridge,
		Prompts:        c.Prompts,
		NowFn:          c.NowFn,
		PhaseIO:        c.PhaseIO,
		CompactPrompts: c.CompactPrompts,
	})
	return &Phase{BaseRunner: base}
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
