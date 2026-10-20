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

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Hooks interface {
	PhaseName() string

	AgentPromptName() string

	ArtifactFilename(req core.PhaseRequest) string

	DefaultModel() string

	ComposePrompt(agentBody string, req core.PhaseRequest) string

	Classify(artifact string, req core.PhaseRequest, bres core.BridgeResponse) (verdict string, diagnostics []core.Diagnostic, nextPhase string)
}

type Skipper interface {
	ShouldSkip(req core.PhaseRequest) (skipped bool, verdict, nextPhase string, diags []core.Diagnostic)
}

type SecondaryArtifactsProvider interface {
	SecondaryArtifacts(req core.PhaseRequest) []string
}

func secondaryArtifacts(h Hooks, req core.PhaseRequest) []string {
	if sp, ok := h.(SecondaryArtifactsProvider); ok {
		return sp.SecondaryArtifacts(req)
	}
	return nil
}

type InlinePromptProvider interface {
	InlinePromptBody() (string, bool)
}

type Options struct {
	Hooks               Hooks
	Bridge              core.Bridge
	Prompts             *prompts.Loader
	NowFn               func() time.Time
	ResolveLLM          func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	StdoutFilter        func(workspace, phase string) error
	EventsProducer      func(workspace, phase, cli string, cycle int, prompt string) error
	Optional            bool
	VerifyFn            func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	ContractVerifier    func() ContractVerifier
	HostEffects         func() core.HostEffects
	SleepFn             func(time.Duration)
	PhaseIO             config.Stage
	CompactPrompts      bool
	DisableStdoutFilter bool
	Router              *cliroute.Router
	Diag                log.Console
	Signals             func() *signalcenter.Center
	GitExec             sysexec.RunFunc
	AttemptClock        func() time.Time
	RetractOnFail       bool
}

type ContractVerifier interface {
	VerifyForClassification(check gatesignal.Check, phase string, roots phasecontract.Roots) (deliverable.Result, error)
}

type BaseRunner struct {
	hooks            Hooks
	bridge           core.Bridge
	prompts          *prompts.Loader
	nowFn            func() time.Time
	resolveLLM       func(phase string, opts resolvellm.Options) (resolvellm.Result, error)
	eventsProducer   func(workspace, phase, cli string, cycle int, prompt string) error
	compactPrompts   bool
	router           *cliroute.Router
	diag             log.Console
	contractVerifier func() ContractVerifier
	hostEffects      func() core.HostEffects
	signals          func() *signalcenter.Center
	verifyInjected   bool
	judge            *verdict.Engine
	gitExec          sysexec.RunFunc
	attemptClock     func() time.Time
	retractOnFail    bool
}

func New(opts Options) *BaseRunner {
	if opts.Hooks == nil {
		panic("phases/runner: Hooks required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
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
	b := &BaseRunner{
		hooks:          opts.Hooks,
		bridge:         opts.Bridge,
		prompts:        opts.Prompts,
		nowFn:          nowFn,
		resolveLLM:     opts.ResolveLLM,
		eventsProducer: eventsProducer,
		compactPrompts: opts.CompactPrompts,
		router:         opts.Router,
		diag:           diag,
	}
	b.judge = wiredVerdictEngine(opts)
	b.contractVerifier = opts.ContractVerifier
	b.hostEffects = opts.HostEffects
	b.signals = resolveSignals(opts)
	b.verifyInjected = opts.VerifyFn != nil
	b.gitExec, b.attemptClock = opts.GitExec, opts.AttemptClock
	b.retractOnFail = opts.RetractOnFail
	if b.gitExec == nil {
		b.gitExec = sysexec.DefaultRunner
	}
	if b.attemptClock == nil {
		b.attemptClock = time.Now
	}
	return b
}

var settleSleep = time.Sleep

func (b *BaseRunner) Name() string { return b.hooks.PhaseName() }

func (b *BaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	req.WorktreeVerified = false
	prep, early, err := b.preparePhaseExecution(req)
	if err != nil {
		return failedPhase(early, err)
	}
	if early != nil {
		return *early, nil
	}

	dispatchPlan, early, err := b.resolveDispatchPlan(req, prep)
	if err != nil {
		return failedPhase(early, err)
	}
	retractable := b.takeRetraction(ctx, req)
	dispatchResult := b.dispatchPhaseAttempts(ctx, req, prep, dispatchPlan)
	if errors.Is(dispatchResult.bridgeErr, bridgechain.ErrUnlaunched) {
		return b.unlaunchedFailure(req, prep, dispatchResult)
	}
	req.WorktreeVerified = dispatchResult.worktreeVerified

	d := dispatchOf(req, prep, dispatchPlan, dispatchResult)
	b.performHostEffects(ctx, d)
	resp, err := b.judge.Judge(ctx, d, b.classifyWith(req, d, dispatchResult.skills))
	return retractable.apply(context.WithoutCancel(ctx), prep.phase, prep.artifactPath, req.Workspace, resp), err
}

func failedPhase(early *core.PhaseResponse, err error) (core.PhaseResponse, error) {
	if early != nil {
		return *early, err
	}
	return core.PhaseResponse{}, err
}

func (b *BaseRunner) unlaunchedFailure(req core.PhaseRequest, prep phasePreparation, d phaseDispatchResult) (core.PhaseResponse, error) {
	resp := core.PhaseResponse{
		Phase: prep.phase, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace,
		DurationMS:  d.durationMS,
		Diagnostics: append(d.fenceDiagnostics, core.Diagnostic{Severity: "error", Message: d.bridgeErr.Error()}),
	}
	return resp, fmt.Errorf("%s: %w", prep.phase, d.bridgeErr)
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

const cycleContextBoundary = "\n\n## Cycle Context\n"

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

func StaticPrefix(prompt string) string {
	if i := strings.Index(prompt, cycleContextBoundary); i >= 0 {
		return prompt[:i]
	}
	return prompt
}

func FormatDigestShadowLog(phase string, record digest.ShadowRecord) string {
	return fmt.Sprintf("[runner] phase=%s digest-shadow outcome=%s full_bytes=%d digest_bytes=%d parity=%t",
		phase, record.Outcome, record.FullBytes, record.DigestBytes, record.Parity)
}

func (b *BaseRunner) ComposePrompt(body string, req core.PhaseRequest) string {
	return b.hooks.ComposePrompt(body, req)
}

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
