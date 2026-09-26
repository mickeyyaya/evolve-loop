// Package advisor is the phase advisor: the LLM brain behind router.Proposer
// and router.Planner, whose every output the router clamp re-validates.
// See docs/architecture/packages/internal-core-advisor.md.
package advisor

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The six WARN codes of the routing/plan decision, registered with their reasons below.
const (
	CodeLaunchFailed        signalcenter.Code = "ADVISOR_LAUNCH_FAILED"
	CodeResponseUnparseable signalcenter.Code = "ADVISOR_RESPONSE_UNPARSEABLE"
	CodeMintRejected        signalcenter.Code = "ADVISOR_MINT_REJECTED"
	CodeProfileLoadFailed   signalcenter.Code = "ADVISOR_PROFILE_LOAD_FAILED"
	CodeReconGitFailed      signalcenter.Code = "ADVISOR_RECON_GIT_FAILED"
	CodeCaptureWriteFailed  signalcenter.Code = "ADVISOR_CAPTURE_WRITE_FAILED"
)

// The fields.step, fields.cause and fields.op vocabularies. The emit sites and the
// registered reasons both build from these, so the stream and signal-codes.md cannot drift.
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
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeProfileLoadFailed, ".evolve/profiles/router.json exists but could not be read or parsed (absence is silent); the dispatch degrades to the single primary CLI exactly as before, so a configured fallback chain is silently narrower than the operator believes; fields.step="+stepDispatch+", path, decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeReconGitFailed, "the pre-plan recon's recent-files reader (git log over the project root) failed while the recon digest was on; the digest is composed without file facts and planning proceeds; fields.step="+stepCompose+", project_root, decision, contract")
	signalcenter.RegisterCode(signalcenter.ModuleAdvisor, CodeCaptureWriteFailed, "a redacted capture artifact (advisor-prompt-, advisor-response- or advisor-span-<kind>) could not be persisted (fields.artifact = prompt / response / span; fields.op = "+opMarshal+" / "+opWrite+" — marshal is dormant, a Span always marshals); the decision still returns and the remaining artifacts are still attempted; the ledger binds nothing for an absent capture; fields.step="+stepCapture+", path, decision, contract")
}

// Identity selects which LLM brain answers, shared by the phase and failure advisors (core.AgentIdentity).
type Identity struct {
	CLI        string // resolved from profile and env by the composition root
	Model      string // a tier (fast/balanced/deep/top) or a raw family model
	Profile    string // empty ⇒ derived from RouteInput.ProjectRoot
	Persona    string // persona body (agents/evolve-*.md); empty ⇒ the legacy inline framing
	AgentLabel string // the bridge "Agent" role tag (router / failure-advisor)
}

// Launcher is the leaf-owned dispatch port that core adapts its Bridge to.
type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResponse, error)
}

// LaunchRequest is the fourteen bridge-request fields the launch sets; its json keys match the bridge's.
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

// LaunchResponse is the four bridge-response fields the advisor reads.
type LaunchResponse struct {
	ExitCode   int
	Stdout     string
	DurationMS int64
	Tokens     cyclestate.TokenUsage
}

// ArtifactWriter persists one capture artifact; nil disables the capture.
type ArtifactWriter func(path string, data []byte) error

// ProfileLoader reads the router profile at path and returns any read or parse error.
type ProfileLoader func(path string) (*profiles.Profile, error)

// RecentFiles lists the files recent commits touched under projectRoot; core injects its git reader.
type RecentFiles func(projectRoot string) ([]string, error)

// DepthCheck is the recursion-depth guard: true for the dispatch env refuses the launch.
type DepthCheck func(env map[string]string) bool

// OverlayResolver names the skill overlays for one attempted CLI.
type OverlayResolver func(cli string) []string

// Advisor owns the routing and plan decisions.
type Advisor struct {
	launcher      Launcher
	identity      Identity
	writeArtifact ArtifactWriter
	loadProfile   ProfileLoader
	recentFiles   RecentFiles
	checkDepth    DepthCheck
	overlays      OverlayResolver
	signals       func() *signalcenter.Center
}

// Option configures an Advisor at construction.
type Option func(*Advisor)

// New builds an Advisor; a nil launcher fails every launch with "nil bridge", the fail-safe the orchestrator degrades on.
func New(launcher Launcher, identity Identity, capture ArtifactWriter, opts ...Option) *Advisor {
	a := &Advisor{
		launcher:      launcher,
		identity:      identity,
		writeArtifact: capture,
		loadProfile:   defaultProfileLoader,
		recentFiles:   func(string) ([]string, error) { return nil, nil },
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// WithProfileLoader replaces the router-profile reader.
func WithProfileLoader(load ProfileLoader) Option {
	return func(a *Advisor) {
		if load != nil {
			a.loadProfile = load
		}
	}
}

// WithRecentFiles replaces the recon's recent-files reader.
func WithRecentFiles(read RecentFiles) Option {
	return func(a *Advisor) {
		if read != nil {
			a.recentFiles = read
		}
	}
}

// WithDepthCheck installs the recursion-depth guard.
func WithDepthCheck(check DepthCheck) Option {
	return func(a *Advisor) { a.checkDepth = check }
}

// WithOverlayResolver replaces the per-attempt skill-overlay resolver.
func WithOverlayResolver(resolve OverlayResolver) Option {
	return func(a *Advisor) {
		if resolve != nil {
			a.overlays = resolve
		}
	}
}

// WithSignals installs the Signal Center accessor, read at every use; a nil accessor or Center reports nothing.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(a *Advisor) { a.signals = c }
}

// SignalsWired reports whether the advisor currently reaches a Center.
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

// Propose implements router.Proposer: the per-transition advice on the next phase and optional inserts.
func (a *Advisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	resp, err := a.launch(in, decisionProposal, buildRoutingPrompt(in))
	if err != nil {
		return nil, err
	}
	prop, err := ParseProposal(resp.Stdout)
	if err != nil {
		a.warnUnparseable(in, decisionProposal, err, resp)
		return nil, fmt.Errorf("%s: %w", decisionProposal.errPfx(), err)
	}
	return prop, nil
}

// Plan implements router.Planner: the advisory upfront whole-cycle run/skip plan.
// See ADR-0024.
func (a *Advisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionPlan)
}

// RePlan is the advisory whole-cycle re-plan once scout's handoff has populated in.Signals.
func (a *Advisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionRePlan)
}

// plan composes the prompt before the launch guards run, so a recon fault reports ahead of a preflight refusal.
func (a *Advisor) plan(in router.RouteInput, d decision) (*router.PhasePlan, error) {
	resp, err := a.launch(in, d, a.composePlanPrompt(in, d, d.artifactFile()))
	if err != nil {
		return nil, err
	}
	parsed, err := ParsePhasePlan(resp.Stdout)
	if err != nil {
		a.warnUnparseable(in, d, err, resp)
		return nil, fmt.Errorf("%s: %w", d.errPfx(), err)
	}
	for _, r := range parsed.RejectedMints {
		a.warn(in, d, CodeMintRejected, r.Reason, map[string]string{"step": stepMint, "minted_phase": r.Phase})
	}
	return parsed.Plan, nil
}
