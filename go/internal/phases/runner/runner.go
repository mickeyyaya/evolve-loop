// Package runner provides BaseRunner, the Template Method skeleton every
// subagent-dispatching phase shares: prompt, dispatch, host effects, judgement.
// See docs/architecture/packages/internal-phases-runner.md.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/digest"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Hooks holds the per-phase variation points BaseRunner delegates to.
type Hooks interface {
	// PhaseName returns the canonical phase id: the profile, bridge Agent and model-key name.
	PhaseName() string

	// AgentPromptName returns the agent doc to load, e.g. "evolve-builder".
	AgentPromptName() string

	// ArtifactFilename returns the contracted artifact's name relative to req.Workspace.
	ArtifactFilename(req core.PhaseRequest) string

	// DefaultModel returns the model used when neither the per-agent MODEL env nor the profile sets one.
	DefaultModel() string

	// ComposePrompt assembles the final prompt from the pre-loaded agent doc body.
	ComposePrompt(agentBody string, req core.PhaseRequest) string

	// Classify judges the artifact; the engine handles a bridge error or missing artifact before calling it.
	Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (verdict string, diagnostics []core.Diagnostic, nextPhase string)
}

// Skipper is an optional Hooks extension: a skip returns its response without touching the bridge.
type Skipper interface {
	ShouldSkip(req core.PhaseRequest) (skipped bool, verdict, nextPhase string, diags []core.Diagnostic)
}

// SecondaryArtifactsProvider is an optional Hooks extension naming deliverables beyond the primary
// artifact; the bridge holds phase-complete until each exists.
type SecondaryArtifactsProvider interface {
	SecondaryArtifacts(req core.PhaseRequest) []string
}

func secondaryArtifacts(h Hooks, req core.PhaseRequest) []string {
	if sp, ok := h.(SecondaryArtifactsProvider); ok {
		return sp.SecondaryArtifacts(req)
	}
	return nil
}

// InlinePromptProvider is an optional Hooks extension: ok=true supplies the prompt body in-band, so no agent doc is read.
type InlinePromptProvider interface {
	InlinePromptBody() (string, bool)
}

// Options configures New; Hooks is required and every nil seam takes its production default.
type Options struct {
	Hooks   Hooks
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// ResolveLLM expands the "auto" model sentinel; nil means resolvellm.Resolve.
	ResolveLLM func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	// StdoutFilter writes the post-phase .clean.txt; nil means logfilter.Process.
	StdoutFilter func(workspace, phase string) error
	// EventsProducer writes <phase>-events.ndjson and has no off switch, because cost and classification read it.
	// prompt feeds the classifier's echo veto, so prompt text an agent quotes never reads as an infra failure.
	EventsProducer func(workspace, phase, cli string, cycle int, prompt string) error
	// Optional degrades an artifact timeout to a WARN that lets the cycle advance; false hard-fails.
	Optional bool
	// VerifyFn replaces the deliverable probe in tests and outranks ContractVerifier.
	VerifyFn func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	// ContractVerifier supplies the deliverables gate's own verifier, so the engine classifies the bytes the gate
	// approves. It is an accessor because the gate is built after the runners; nil falls back to the catalog-aware verify.
	ContractVerifier func() ContractVerifier
	// HostEffects performs the phase's declared host effects before the verdict
	// engine judges it; an accessor for the same reason as ContractVerifier.
	HostEffects func() core.HostEffects
	// SleepFn is the delay between the engine's settle retries; nil means settleSleep.
	SleepFn func(time.Duration)
	// PhaseIO is the EVOLVE_PHASE_IO stage the default probe honors, as the host gate does.
	PhaseIO config.Stage
	// CompactPrompts strips on-demand reference sections from disk-loaded agent docs; inline bodies are never stripped.
	CompactPrompts      bool
	DisableStdoutFilter bool
	// UniversalFallback appends discovered CLIs when no CLI of the configured chain is installed; inert without DiscoverCLIsFn.
	UniversalFallback bool
	// DiscoverCLIsFn lists the installed, authed, non-blocked drivers; a seam so this package never imports bridge.
	DiscoverCLIsFn func() []string
	// Diag receives the routing-overlay observability lines; the zero value means log.Diag().
	Diag log.Console
	// Signals is the engine's Signal Center accessor; nil adopts the Bridge's own Center when it exposes one.
	Signals func() *signalcenter.Center
}

// ContractVerifier is the deliverables gate's verification, salvage included, offered to the verdict engine.
type ContractVerifier interface {
	VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (deliverable.Result, error)
}

// BaseRunner is the Template Method phase runner, a core.PhaseRunner built by New.
type BaseRunner struct {
	hooks             Hooks
	bridge            core.Bridge
	prompts           *prompts.Loader
	nowFn             func() time.Time
	resolveLLM        func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	eventsProducer    func(workspace, phase, cli string, cycle int, prompt string) error
	compactPrompts    bool
	universalFallback bool
	discoverCLIsFn    func() []string
	diag              log.Console
	contractVerifier  func() ContractVerifier
	hostEffects       func() core.HostEffects
	signals           func() *signalcenter.Center
	verifyInjected    bool
	judge             *verdict.Engine
}

// New constructs a BaseRunner and panics on nil Hooks, a wiring error caught at startup.
func New(opts Options) *BaseRunner {
	if opts.Hooks == nil {
		panic("phases/runner: Hooks required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	resolveLLM := opts.ResolveLLM
	if resolveLLM == nil {
		resolveLLM = resolvellm.Resolve
	}
	eventsProducer := opts.EventsProducer
	if eventsProducer == nil {
		eventsProducer = func(workspace, phase, cli string, cycle int, prompt string) error {
			return phasestream.Produce(phasestream.ProduceConfig{
				Workspace: workspace, Phase: phase, CLI: cli, Cycle: cycle,
				InjectedPrompt: prompt,
			})
		}
	}
	diag := opts.Diag
	if diag.Out == nil && diag.Err == nil {
		diag = log.Diag()
	}
	universalFallback := opts.UniversalFallback || DefaultUniversalFallback
	discoverCLIsFn := opts.DiscoverCLIsFn
	if discoverCLIsFn == nil {
		discoverCLIsFn = DefaultDiscoverCLIsFn
	}
	b := &BaseRunner{
		hooks:             opts.Hooks,
		bridge:            opts.Bridge,
		prompts:           opts.Prompts,
		nowFn:             nowFn,
		resolveLLM:        resolveLLM,
		eventsProducer:    eventsProducer,
		compactPrompts:    opts.CompactPrompts,
		universalFallback: universalFallback,
		discoverCLIsFn:    discoverCLIsFn,
		diag:              diag,
	}
	b.judge = wiredVerdictEngine(opts)
	b.contractVerifier = opts.ContractVerifier
	b.hostEffects = opts.HostEffects
	b.signals = resolveSignals(opts)
	b.verifyInjected = opts.VerifyFn != nil
	return b
}

// settleSleep is a variable so the package's tests can make the settle window free: it sits on the clean-exit path.
var settleSleep = time.Sleep

// Name implements core.PhaseRunner.
func (b *BaseRunner) Name() string { return b.hooks.PhaseName() }

// Run implements core.PhaseRunner: prepare, plan, dispatch inside the worktree fence, perform host effects, judge.
func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	// Only this run's fence may vouch for the tree, never the caller.
	req.WorktreeVerified = false
	prep, early, err := b.preparePhaseExecution(req)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	if early != nil {
		return *early, nil
	}

	dispatchPlan, early, err := b.resolveDispatchPlan(req, prep)
	if err != nil {
		if early != nil {
			return *early, err
		}
		return core.PhaseResponse{}, err
	}
	dispatchResult := b.dispatchPhaseAttempts(ctx, req, prep, dispatchPlan)
	req.WorktreeVerified = dispatchResult.worktreeVerified

	d := dispatchOf(req, prep, dispatchPlan, dispatchResult)
	// Effects first: the judge's first verification must see what the host performed.
	b.performHostEffects(ctx, d)
	return b.judge.Judge(ctx, d, b.classifyWith(req, dispatchResult.bridgeResponse))
}

func (b *BaseRunner) withExplanationContract(body, phase string) (string, error) {
	var agentName, heading string
	switch phase {
	case string(core.PhaseBuild):
		agentName = "evolve-builder-reference"
		heading = "## Section: explanation-documentation-contract"
	case string(core.PhaseAudit):
		agentName = "evolve-auditor-reference"
		heading = "## Section: explanation-documentation-review"
	default:
		return body, nil
	}
	if strings.Contains(body, heading) {
		return body, nil
	}
	agent, err := b.prompts.Agent(agentName)
	if err != nil {
		return "", err
	}
	start := strings.Index(agent.Body, heading)
	if start < 0 {
		return "", fmt.Errorf("%s is missing %q", agentName, heading)
	}
	section := agent.Body[start:]
	if next := strings.Index(section[len(heading):], "\n## Section:"); next >= 0 {
		section = section[:len(heading)+next]
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.TrimSpace(section) + "\n", nil
}

func requiresExplanationSandbox(phase string, req core.PhaseRequest) bool {
	return phase == string(core.PhaseBuild) && req.ExplanationDocumentationVersion != 0
}

// cycleContextBoundary splits a prompt's cache-stable prefix from its per-cycle tail. One literal serves
// BaseCycleContext and StaticPrefix, so they cannot drift apart and bust the provider prompt cache.
const cycleContextBoundary = "\n\n## Cycle Context\n"

// BaseCycleContext appends the shared "## Cycle Context" block to body; phases append their own fields after it.
func BaseCycleContext(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString(cycleContextBoundary)
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if recalled := req.Context[core.CtxKeyRecallMemory]; recalled != "" {
		fmt.Fprintf(&b, "- recalled_lessons_untrusted_json: %q\n", recalled)
	}
	AppendExplanationContext(&b, req)
	return b.String()
}

// AppendExplanationContext writes the verified Build-explanation handoff. Audit and Retro share it,
// so their prompts carry exactly the evidence their report validators require.
func AppendExplanationContext(b *strings.Builder, req core.PhaseRequest) {
	if req.ExplanationDocumentationVersion != 0 {
		fmt.Fprintf(b, "- explanation_documentation_version: %d\n", req.ExplanationDocumentationVersion)
	}
	if req.BuildExplanationState != "" {
		fmt.Fprintf(b, "- explanation_handoff_state: %s\n", req.BuildExplanationState)
	}
	if req.BuildExplanationError != "" {
		fmt.Fprintf(b, "- explanation_error_untrusted_json: %q\n", req.BuildExplanationError)
	}
	if explanation := req.BuildExplanation; explanation != nil {
		fmt.Fprintf(b, "- explanation_status: %s\n", explanation.Status)
		fmt.Fprintf(b, "- explanation_contract_version: %d\n", explanation.ContractVersion)
		fmt.Fprintf(b, "- explanation_document: %s (sha256:%s)\n", explanation.DocumentPath, explanation.DocumentSHA256)
		if encoded, err := json.Marshal(explanation); err == nil {
			fmt.Fprintf(b, "- explanation_handoff_untrusted_json: %s\n", encoded)
		}
	}
}

// StaticPrefix returns the prompt before the Cycle Context boundary, the prefix provider prompt caches key on.
func StaticPrefix(prompt string) string {
	if i := strings.Index(prompt, cycleContextBoundary); i >= 0 {
		return prompt[:i]
	}
	return prompt
}

// FormatDigestShadowLog renders the role projection's dispatch-time evidence.
func FormatDigestShadowLog(phase string, record digest.ShadowRecord) string {
	return fmt.Sprintf("[runner] phase=%s digest-shadow outcome=%s full_bytes=%d digest_bytes=%d parity=%t",
		phase, record.Outcome, record.FullBytes, record.DigestBytes, record.Parity)
}

// ComposePrompt runs the phase's prompt hook without dispatching, for the cache-stable-prefix audit.
func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}

// PersonaAvailable implements core.PersonaProber, resolving the persona doc exactly as Run does without dispatching.
func (b *BaseRunner) PersonaAvailable() error {
	if ip, ok := b.hooks.(InlinePromptProvider); ok {
		if _, inline := ip.InlinePromptBody(); inline {
			return nil
		}
	}
	if b.prompts == nil {
		return fmt.Errorf("%s: prompts loader required", b.hooks.PhaseName())
	}
	if _, err := b.prompts.Agent(b.hooks.AgentPromptName()); err != nil {
		if errors.Is(err, fs.ErrNotExist) && !errors.Is(err, prompts.ErrNoSource) {
			return fmt.Errorf("%s: load agent: %w: %w", b.hooks.PhaseName(), core.ErrAgentDocMissing, err)
		}
		return fmt.Errorf("%s: load agent: %w", b.hooks.PhaseName(), err)
	}
	return nil
}
