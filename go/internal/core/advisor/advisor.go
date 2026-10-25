// Package advisor is the phase advisor: the LLM brain behind router.Proposer
// and router.Planner, whose every output the router clamp re-validates.
// See docs/architecture/packages/internal-core-advisor.md.
package advisor

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeLaunchFailed        signalcenter.Code = "ADVISOR_LAUNCH_FAILED"
	CodeResponseUnparseable signalcenter.Code = "ADVISOR_RESPONSE_UNPARSEABLE"
	CodeMintRejected        signalcenter.Code = "ADVISOR_MINT_REJECTED"
	CodeReconGitFailed      signalcenter.Code = "ADVISOR_RECON_GIT_FAILED"
	CodeCaptureWriteFailed  signalcenter.Code = "ADVISOR_CAPTURE_WRITE_FAILED"
)

const (
	stepPreflight = "preflight"
	stepDispatch  = "dispatch"
	stepParse     = "parse"
	stepMint      = "mint"
	stepCompose   = "compose"
	stepCapture   = "capture"

	causeNoJSON      = "no_json"
	causeInvalidJSON = "invalid_json"
	causeEmpty       = "empty"

	opMarshal = "marshal"
	opWrite   = "write"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeLaunchFailed, "the routing/plan dispatch produced no response — the preflight refused (nil bridge, empty workspace, the depth guard; fields.step="+stepPreflight+") or every CLI in the router profile's fallback chain failed (fields.step="+stepDispatch+", cli, chain, exit_code, profile); the error is still returned and the caller degrades to the static spine or keeps the initial plan; fields.decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeResponseUnparseable, "the launch returned but no decision decoded from its output (fields.cause = "+causeNoJSON+" / "+causeInvalidJSON+" / "+causeEmpty+"); the wrapped error is still returned and the caller degrades — on the per-transition Propose path this is the first visibility the fault ever had; fields.step="+stepParse+", stdout_bytes, artifact, decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeMintRejected, "a plan entry minted a reserved control-plane identity (router/advisor/failure-advisor and their aliases) and was dropped by the recursion guard; the rest of the plan stands, one event per drop at decision time (never on resume or replay); fields.step="+stepMint+", minted_phase, decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeReconGitFailed, "the pre-plan recon's recent-files reader (git log over the project root) failed while the recon digest was on; the digest is composed without file facts and planning proceeds; fields.step="+stepCompose+", project_root, decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeCaptureWriteFailed, "a redacted capture artifact (advisor-prompt-, advisor-response- or advisor-span-<kind>) could not be persisted (fields.artifact = prompt / response / span; fields.op = "+opMarshal+" / "+opWrite+" — marshal is dormant, a Span always marshals); the decision still returns and the remaining artifacts are still attempted; the ledger binds nothing for an absent capture; fields.step="+stepCapture+", path, decision, contract")
}

type Identity struct {
	CLI        string
	Model      string
	Profile    string
	Persona    string
	AgentLabel string
}

type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResponse, error)
}

type LaunchRequest struct {
	CLI          string            `json:"cli"`
	Profile      string            `json:"profile"`
	Model        string            `json:"model"`
	Skills       []string          `json:"skills,omitempty"`
	Prompt       string            `json:"prompt"`
	Workspace    string            `json:"workspace"`
	Worktree     string            `json:"worktree,omitempty"`
	ProjectRoot  string            `json:"project_root,omitempty"`
	ArtifactPath string            `json:"artifact_path,omitempty"`
	Completion   string            `json:"completion,omitempty"`
	Agent        string            `json:"agent,omitempty"`
	Contract     string            `json:"contract,omitempty"`
	Cycle        int               `json:"cycle,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type LaunchResponse struct {
	ExitCode   int
	Stdout     string
	DurationMS int64
	Tokens     cyclestate.TokenUsage
}

type ArtifactWriter func(path string, data []byte) error

type RecentFiles func(projectRoot string) ([]string, error)

type DepthCheck func(env map[string]string) bool

type OverlayResolver func(cli string) []string

type Advisor struct {
	launcher      Launcher
	identity      Identity
	writeArtifact ArtifactWriter
	recentFiles   RecentFiles
	checkDepth    DepthCheck
	overlays      OverlayResolver
	signals       func() *signalcenter.Center
	route         llmroute.Plan
}

type Option func(*Advisor)

func New(launcher Launcher, identity Identity, capture ArtifactWriter, opts ...Option) *Advisor {
	a := &Advisor{
		launcher:      launcher,
		identity:      identity,
		writeArtifact: capture,
		recentFiles:   func(string) ([]string, error) { return nil, nil },
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithRoute(walk llmroute.Plan) Option {
	return func(a *Advisor) { a.route = walk }
}

func WithRecentFiles(read RecentFiles) Option {
	return func(a *Advisor) {
		if read != nil {
			a.recentFiles = read
		}
	}
}

func WithDepthCheck(check DepthCheck) Option {
	return func(a *Advisor) { a.checkDepth = check }
}

func WithOverlayResolver(resolve OverlayResolver) Option {
	return func(a *Advisor) {
		if resolve != nil {
			a.overlays = resolve
		}
	}
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(a *Advisor) { a.signals = c }
}

func (a *Advisor) SignalsWired() bool { return a.center() != nil }

func (a *Advisor) center() *signalcenter.Center {
	if a.signals == nil {
		return nil
	}
	return a.signals()
}

func (a *Advisor) warn(in router.RouteInput, d decision, code signalcenter.Code, reason string, fields map[string]string) {
	fields["decision"] = d.captureKind()
	fields["contract"] = d.contractID()
	a.center().Emit(signalcenter.Event{
		Cycle: in.Cycle, Phase: in.Current, Module: signalcenter.ModuleAdvisor, Origin: d.origin(),
		Kind: signalcenter.KindAdvisorWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

func (a *Advisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	resp, err := a.launch(in, decisionProposal, buildRoutingPrompt(in))
	if err != nil {
		return nil, err
	}
	prop, err := DecodeProposal(a.decisionBytes(in, decisionProposal, StdoutSpan(resp.Stdout, '{', '}')))
	if err != nil {
		a.warnUnparseable(in, decisionProposal, err, resp)
		return nil, fmt.Errorf("%s: %w", decisionProposal.errPfx(), err)
	}
	return prop, nil
}

func (a *Advisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionPlan)
}

func (a *Advisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionRePlan)
}

func (a *Advisor) decisionBytes(in router.RouteInput, d decision, legacy func() ([]byte, bool)) []byte {
	cell := phasecontract.FlowCell{Cycle: in.Cycle, Phase: in.Current, Origin: d.origin(), Path: filepath.Join(in.Workspace, d.artifactFile())}
	return phasecontract.MustFlowDecision(d.flowSeat()).Resolve(in.ProjectRoot, cell, legacy, a.center())
}

func (a *Advisor) plan(in router.RouteInput, d decision) (*router.PhasePlan, error) {
	resp, err := a.launch(in, d, a.composePlanPrompt(in, d, d.artifactFile()))
	if err != nil {
		return nil, err
	}
	parsed, err := DecodePhasePlan(a.decisionBytes(in, d, StdoutSpan(resp.Stdout, '[', ']')))
	if err != nil {
		a.warnUnparseable(in, d, err, resp)
		return nil, fmt.Errorf("%s: %w", d.errPfx(), err)
	}
	for _, r := range parsed.RejectedMints {
		a.warn(in, d, CodeMintRejected, r.Reason, map[string]string{"step": stepMint, "minted_phase": r.Phase})
	}
	return parsed.Plan, nil
}
