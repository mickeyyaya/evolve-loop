// Package advisor is unit 04 of the component breakdown (ADR-0103): the phase
// advisor — the DynamicLLM brain behind router.Proposer and router.Planner.
// One Advisor composes the per-transition routing prompt and the whole-cycle
// plan prompt over router.RouteInput, launches the router persona through a
// leaf-owned Launcher port (core adapts its Bridge to it once), walks the
// router profile's CLI fallback chain, persists the redacted prompt/response
// capture and the OTel-GenAI decision span, and parses the strict-JSON
// proposal or plan with the mint recursion guard. Every output is ADVISORY:
// the pure router clamp re-validates it against the kernel floor, and any
// failure is returned as an error so the caller degrades to the static path.
// The Advisor holds its collaborators explicitly — the launcher, the identity,
// the capture writer, the profile loader, the recent-files reader, the depth
// guard, the overlay resolver and the Signal Center accessor — never runs git,
// never writes stderr, and reports its six failure modes as advisor.warning
// under module advisor. Design: docs/architecture/decomposition/04-advisor.md.
package advisor

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes — the six WARN conditions of the routing/plan decision,
// registered with their reasons. One replaced the file's only stderr line
// (the dropped mint); the others made five silent fault paths visible.
const (
	CodeLaunchFailed        signalcenter.Code = "ADVISOR_LAUNCH_FAILED"
	CodeResponseUnparseable signalcenter.Code = "ADVISOR_RESPONSE_UNPARSEABLE"
	CodeMintRejected        signalcenter.Code = "ADVISOR_MINT_REJECTED"
	CodeProfileLoadFailed   signalcenter.Code = "ADVISOR_PROFILE_LOAD_FAILED"
	CodeReconGitFailed      signalcenter.Code = "ADVISOR_RECON_GIT_FAILED"
	CodeCaptureWriteFailed  signalcenter.Code = "ADVISOR_CAPTURE_WRITE_FAILED"
)

// The closed vocabularies of the event fields a triage filters on — fields.step
// (where in the decision the fault sat), fields.cause (why nothing decoded)
// and fields.op (which capture step failed) — spelled ONCE: the emit sites and
// the registered reasons below are built from these, so the stream contract
// and the generated signal-codes.md cannot drift apart.
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

// Identity is the immutable dispatch identity shared by the control-plane
// advisors (the phase advisor, the failure advisor) — the fields that select
// WHICH llm brain answers, independent of the per-call operand (prompt /
// artifact file / completion contract, which vary Plan vs Propose vs Advise
// and stay per-call). It formalizes the byte-identical {cli,model,profile,
// persona} field set both advisors carried separately (ADR-0052 WS1-S1, Value
// Object): one home per identity belief, never two structs drifting apart.
// core.AgentIdentity is an alias of it.
//
// It is deliberately NOT the bridge-launch call itself — the advisors thread
// context differently (the phase advisor uses context.Background; the failure
// advisor threads the caller's ctx) — so only the field-set used to build the
// launch request is shared.
type Identity struct {
	CLI        string // dispatch CLI (claude-tmux / codex / agy); resolved from profile + env by the composition root
	Model      string // model tier requested (haiku/sonnet/opus or a raw family model)
	Profile    string // profile path; when empty the advisor derives it from RouteInput.ProjectRoot
	Persona    string // persona body (agents/evolve-*.md); empty ⇒ legacy inline framing
	AgentLabel string // bridge "Agent" role tag (router / failure-advisor)
}

// Launcher is the leaf-owned dispatch port: core adapts its Bridge to it once
// (the seam's bridgeLauncher), so the brain never names a core type.
type Launcher interface {
	Launch(ctx context.Context, req LaunchRequest) (LaunchResponse, error)
}

// LaunchRequest is the advisor's consumed subset of the bridge request —
// exactly the fourteen fields the launch sets. The json keys match the
// bridge's so the pre-extraction launch goldens replay through it; tests
// construct it POSITIONALLY so a new field is a compile error in the adapter.
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

// ArtifactWriter persists one capture artifact; nil is the Null Object (no
// forensics). Core injects its atomic temp-file-plus-rename writer, which is
// what keeps the ledger's disk-read SHA equal to the span's in-memory SHA.
type ArtifactWriter func(path string, data []byte) error

// ProfileLoader reads the router profile at path; the default parses it
// through profiles.NewFromDir(dir).Get(name) and RETURNS the error (the
// advisor classifies absence as silent and everything else as a fault).
type ProfileLoader func(path string) (*profiles.Profile, error)

// RecentFiles lists the files touched by recent commits under projectRoot for
// the pre-plan recon; the default is the Null Object (nil, nil) — core injects
// its git reader, so the leaf never runs git.
type RecentFiles func(projectRoot string) ([]string, error)

// DepthCheck is the injectable recursion-depth guard (defense-in-depth,
// ADR-0052 §4.3): true for the dispatch env refuses the launch. nil skips it.
type DepthCheck func(env map[string]string) bool

// OverlayResolver names the skill overlays for one attempted CLI; the default
// resolves the compiled policy defaults for the identity's tier (deep/top →
// fable), exactly as the pre-extraction launch did.
type OverlayResolver func(cli string) []string

// Advisor owns the routing/plan decision. Every collaborator is explicit at
// construction; the Center is read through an accessor at every use.
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

// Option configures an Advisor at construction (functional options); a nil
// value keeps the default.
type Option func(*Advisor)

// New builds the brain over its required collaborators. A nil launcher is
// LEGAL and deliberately not a Null Object: every launch then fails with
// "<prefix>: nil bridge", the fail-safe the orchestrator degrades on.
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

// WithOverlayResolver replaces the per-attempt skill-overlay resolver. A DI
// seam with NO production caller today: the composition root does not hand
// the loaded policy's ResolveOverlays in (the zero-policy default below is
// the preserved pre-extraction behaviour — operator question 1 in the unit
// doc §8); the leaf tests use it to observe the per-attempt resolution.
func WithOverlayResolver(resolve OverlayResolver) Option {
	return func(a *Advisor) {
		if resolve != nil {
			a.overlays = resolve
		}
	}
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use. A nil accessor, or one returning nil, is the
// Null Object; SignalsWired proves the production root wired one.
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

// warn is the unit's one producer: an advisor.warning WARN under module
// advisor, stamped with the cycle, the just-completed phase, the decision and
// its contract, from the exported method whose call produced it.
func (a *Advisor) warn(in router.RouteInput, d decision, code signalcenter.Code, reason string, fields map[string]string) {
	fields["decision"] = d.captureKind()
	fields["contract"] = d.contractID()
	a.center().Emit(signalcenter.Event{
		Cycle: in.Cycle, Phase: in.Current, Module: signalcenter.ModuleAdvisor, Origin: d.origin(),
		Kind: signalcenter.KindAdvisorWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

// Propose implements router.Proposer — the per-transition "insert this
// optional phase?" advice under the ADR-0027 stdout completion contract.
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

// Plan implements router.Planner: the upfront whole-cycle run/skip plan
// (ADR-0024 §2 hybrid cadence — the cheap, coherent upfront decision). The
// returned plan is ADVISORY; the kernel clamp re-validates it against the
// floor. It writes routing-plan.json and parses a JSON array; any failure
// returns an error so the caller degrades to the static path.
func (a *Advisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionPlan)
}

// RePlan is the post-scout re-plan (ADR-0052 WS1-S3): a SECOND whole-cycle
// plan computed once scout's handoff has populated in.Signals, so need is
// MEASURED rather than inferred from goal text. It shares plan with the
// initial Plan — same compose→dispatch→parse path — writes the DISTINCT
// routing-replan.json artifact and stamps replan_depth=1 on its decision
// span. The orchestrator calls it in shadow every cycle post-scout under the
// cfg.RouterReplan dial (cyclerun_replan.go); the plan is ADVISORY and the
// kernel clamp re-validates it exactly as it does the initial plan.
func (a *Advisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return a.plan(in, decisionRePlan)
}

// plan is the shared whole-cycle planning wiring: compose (the prompt is
// composed BEFORE the launch guards run, as the pre-extraction argument
// order did) → dispatch → capture → parse → the mint drops reported once,
// stamped with this decision.
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
