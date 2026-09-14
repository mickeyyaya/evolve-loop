package subagent

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

// run.go is the unit-16 seam (ADR-0103): the `evolve subagent run` execution
// path lives in internal/subagent/subagentrun; this file keeps the exported
// RunRequest / RunOptions / RunResult bag the root and the by-name test
// binders spell, the ONE wired construction of the dispatcher from those
// options, the request/result projections, and the facades the fan-out
// dispatcher, the Runner twin, the validate pipeline and the host tests keep.

// RunRequest captures every input cmd_run reads from argv + environment.
// Mirrors the bash signature subagent-run.sh <agent> <cycle> <workspace>
// + env overrides (PROMPT_FILE_OVERRIDE, MODEL_TIER_HINT, WORKTREE_PATH,
// ADVERSARIAL_AUDIT, LEGACY_AGENT_DISPATCH).
type RunRequest struct {
	Agent         string
	Cycle         int
	WorkspacePath string

	ProfilesDir   string
	AdaptersDir   string
	CapabilityDir string
	ProjectRoot   string
	PluginRoot    string
	WorktreePath  string
	LedgerPath    string

	// PromptReader supplies the user task prompt. Caller can pass an
	// *os.File (PROMPT_FILE_OVERRIDE), os.Stdin, or a bytes.Buffer.
	// MUST be non-nil — bash fails fast when no prompt source is configured.
	PromptReader io.Reader

	// Env overrides bash reads at function entry.
	ModelTierHint          string // MODEL_TIER_HINT
	AuditorTierOverride    string // EVOLVE_AUDITOR_TIER_OVERRIDE
	DiffComplexityDisabled bool   // EVOLVE_DIFF_COMPLEXITY_DISABLE=1
	AdversarialAudit       bool   // ADVERSARIAL_AUDIT (default true)
	LegacyAgentDispatch    bool   // LEGACY_AGENT_DISPATCH=1 — retired; now a hard error (see ErrInProcessDispatchBanned)
	DispatchDepth          int    // EVOLVE_DISPATCH_DEPTH — recursion depth; capped at maxDispatchDepth
	// ChallengeTokenOverride (EVOLVE_FANOUT_WORKER_TOKEN) pins the challenge
	// token instead of minting a fresh one. A fan-out worker is dispatched with
	// the parent-dictated token (parentToken+"-"+subtask) so its artifact bears
	// a token the parent can verify — the provenance boundary for per-worker
	// artifact verification. Empty ⇒ mint via Rand (the normal path).
	ChallengeTokenOverride string
}

// ErrInProcessDispatchBanned is returned when a caller requests the retired
// in-process dispatch path (LEGACY_AGENT_DISPATCH=1). The agent-bridge
// (`evolve subagent run`) is the ONE and ONLY supported dispatch path; the
// invariant and its sentinel live in the leaf (the same pointer, so errors.Is
// holds).
var ErrInProcessDispatchBanned = subagentrun.ErrInProcessDispatchBanned

// RunOptions injects the I/O + sub-process seams. Production wires
// defaults; tests substitute doubles.
type RunOptions struct {
	ReadProfile       func(path string) (string, error)
	ResolveLLM        func(agent string) (resolvellm.Result, error)
	InspectCapability func(adaptersDir, cli string) (capability.Inspection, error)
	ResolveModelTier  func(req ResolveModelTierRequest, opts ResolveModelTierOptions) (string, error)
	// AdapterExists reports whether the resolved cli has a registered bridge
	// driver. Since ADR-0103 unit 16 it receives the CLI, not the vestigial
	// <AdaptersDir>/<cli>.sh path (the func type is unchanged; the production
	// default is driverExists — nothing on the run path decodes a file name).
	AdapterExists func(cli string) bool
	ExecAdapter   func(ctx context.Context, adapterPath string, env map[string]string) (exitCode int, err error)
	// WriteFile is unused since the bridge port (nothing on the run path
	// writes through it); kept for the by-name binders that set it.
	//
	// Deprecated: dead seam, retired with the next binder edit (unit 16 follow-up 16-4).
	WriteFile func(path string, data []byte, mode os.FileMode) error
	GitState  func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	StatMTime func(path string) (time.Time, error)
	ReadFile  func(path string) ([]byte, error)
	HashFile  func(path string) (string, error)
	Now       func() time.Time
	Rand      func([]byte) (int, error)
	// Signals is the root's Signal Center (ADR-0103 unit 16): the dispatcher's
	// BRIDGE_SUBAGENT_* warnings and, through the exec seam, the bridge
	// engine's own producers report into it. nil is the Null Object.
	Signals *signalcenter.Center
}

// RunResult carries everything cmd_run printed + the side effects.
// Verdict is one of VerdictPASS / VerdictFAIL / VerdictIntegrityFail.
type RunResult struct {
	Verdict        string
	CLI            string
	Model          string
	ArtifactPath   string
	ArtifactSHA256 string
	ChallengeToken string
	ExitCode       int
	DurationMS     int64
	Warns          []string
	Stderr         string // collected adapter stderr for caller logging
}

// nonRegistryRoles are dispatchable agent roles that ship a
// .evolve/profiles/<role>.json but have NO phasecontract entry: they are not
// spine phases with a report contract, so the registry cannot know them. This
// is the ONLY hand-maintained half of the allow-list, and it is deliberately
// the small half — adding a spine phase to phasecontract now makes it
// dispatchable automatically.
var nonRegistryRoles = []string{
	"inspirer", "evaluator", "plan-reviewer", "memo", "tester",
}

// agentRoles is the canonical allow-list of agent roles (single source of
// truth). agentRolePattern is derived from it, and tests iterate it, so a new
// role is added in exactly one place.
//
// The Go list is the ONLY source: the bash dispatcher it once mirrored is
// retired, and every consumer — the dispatcher's KnownRole port, the fan-out
// gate and the conformance tests — derives from here.
var agentRoles = buildAgentRoles()

// buildAgentRoles derives the allow-list as the UNION of (a) every
// phasecontract-registered agent that actually produces an LLM deliverable and
// (b) nonRegistryRoles. Before cycle-1145 this was a second hand-typed slice
// beside the registry and had already drifted: "router" is registered (and ships
// .evolve/profiles/router.json) yet was not dispatchable.
//
// NoArtifact phases are excluded: "ship" is registered but is a native
// host-side phase with no profile, so accepting it would break the
// role↔profile conformance invariant (TestAgentRoles_EveryRoleHasProfile).
// Output is sorted so the derived regex — and every test that iterates the
// list — is deterministic despite Contracts()' unordered map iteration.
func buildAgentRoles() []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(nonRegistryRoles)+len(phasecontract.Contracts()))
	add := func(role string) {
		if role == "" || seen[role] {
			return
		}
		seen[role] = true
		out = append(out, role)
	}
	for _, c := range phasecontract.Contracts() {
		if c.NoArtifact {
			continue
		}
		add(c.AgentName)
	}
	for _, r := range nonRegistryRoles {
		add(r)
	}
	sort.Strings(out)
	return out
}

// agentRolePattern matches exactly the canonical roles in agentRoles.
var agentRolePattern = regexp.MustCompile(`^(` + strings.Join(agentRoles, "|") + `)$`)

// Run is the `evolve subagent run` execution path: argument validation,
// worker-name parsing, profile load, cli/model resolution, the driver check,
// model tier resolution, artifact placement, the challenge token and git
// state, the prompt (PROMPT_FILE_OVERRIDE or stdin) assembled into the v2
// cache-prefix envelope with the adversarial auditor framing, the adapter
// exec with VALIDATE_ONLY=0 and the full env, artifact verification (exists,
// fresh <5min, token-bearing) and the kind="agent_subprocess" ledger entry.
// The path is internal/subagent/subagentrun (ADR-0103 unit 16); this facade
// fills the production defaults, builds the ONE wired dispatcher and projects
// the result.
func Run(ctx context.Context, req RunRequest, opts RunOptions) (RunResult, error) {
	fillRunDefaults(&opts)
	out, err := wiredDispatcher(opts).Dispatch(ctx, requestOf(req))
	return resultOf(out), err
}

// wiredDispatcher is the ONE construction of the unit-16 dispatcher
// (TestSubagentRun_OneConstructionSite): every host-backed port is the option
// the caller (or fillRunDefaults) supplied, projected once onto the leaf's
// shapes; the stdlib-backed collaborators are the caller's seams; the Center
// is read through an accessor so a nil one stays the Null Object.
func wiredDispatcher(opts RunOptions) *subagentrun.Dispatcher {
	deps := subagentrun.Deps{
		Profile:     profileOf(opts.ReadProfile),
		ResolveLLM:  llmOf(opts.ResolveLLM),
		Inspect:     capabilityOf(opts.InspectCapability),
		ResolveTier: tierOf(opts.ResolveModelTier),

		KnownRole:  agentRolePattern.MatchString,
		GuardDepth: enforceDispatchDepth,

		AdapterExists: opts.AdapterExists,
		Adapter:       adapterOf(opts),
		GitState:      opts.GitState,
		RunID:         core.RunIDFromWorkspace,
	}
	return subagentrun.New(deps,
		subagentrun.WithClock(opts.Now), subagentrun.WithRand(opts.Rand),
		subagentrun.WithFS(opts.StatMTime, opts.ReadFile, opts.HashFile),
		subagentrun.WithSignals(func() *signalcenter.Center { return opts.Signals }),
	)
}

// requestOf is the ONE projection of the root's request onto the leaf's —
// field for field; PluginRoot is never read by the path and does not ride in.
func requestOf(req RunRequest) subagentrun.Request {
	return subagentrun.Request{
		Agent: req.Agent, Cycle: req.Cycle, WorkspacePath: req.WorkspacePath,
		ProfilesDir: req.ProfilesDir, AdaptersDir: req.AdaptersDir, CapabilityDir: req.CapabilityDir,
		ProjectRoot: req.ProjectRoot, WorktreePath: req.WorktreePath, LedgerPath: req.LedgerPath,
		Prompt:        req.PromptReader,
		ModelTierHint: req.ModelTierHint, AuditorTierOverride: req.AuditorTierOverride,
		DiffComplexityDisabled: req.DiffComplexityDisabled, AdversarialAudit: req.AdversarialAudit,
		LegacyAgentDispatch: req.LegacyAgentDispatch, DispatchDepth: req.DispatchDepth,
		ChallengeTokenOverride: req.ChallengeTokenOverride,
	}
}

// resultOf is the ONE projection of the leaf's outcome onto the root's
// result; Stderr is never set (as before) and the verification evidence the
// outcome carries is the signal's, not the result's.
func resultOf(out subagentrun.Outcome) RunResult {
	return RunResult{
		Verdict: out.Verdict, CLI: out.CLI, Model: out.Model,
		ArtifactPath: out.ArtifactPath, ArtifactSHA256: out.ArtifactSHA256, ChallengeToken: out.ChallengeToken,
		ExitCode: out.ExitCode, DurationMS: out.DurationMS, Warns: out.Warns,
	}
}

// profileOf projects the raw-JSON profile grammar (matchField over the body,
// the adapter overrides for the cli resolved later) onto the leaf's Profile.
func profileOf(read func(string) (string, error)) func(string) (subagentrun.Profile, error) {
	return func(path string) (subagentrun.Profile, error) {
		body, err := read(path)
		if err != nil {
			return subagentrun.Profile{}, err
		}
		return subagentrun.Profile{
			CLI:            matchField(body, reFieldCLI),
			OutputArtifact: matchField(body, reFieldOutputArtifact),
			Overrides: func(cli string) (string, string) {
				o := extractAdapterOverrides(body, cli)
				return o.ToolsJSON, o.ExtraFlagsJSON
			},
		}, nil
	}
}

func llmOf(resolve func(string) (resolvellm.Result, error)) func(string) (subagentrun.LLM, error) {
	return func(role string) (subagentrun.LLM, error) {
		r, err := resolve(role)
		return subagentrun.LLM{CLI: r.CLI, ModelTier: r.ModelTier, Source: r.Source}, err
	}
}

func capabilityOf(inspect func(string, string) (capability.Inspection, error)) func(string, string) (subagentrun.Capability, error) {
	return func(dir, cli string) (subagentrun.Capability, error) {
		insp, err := inspect(dir, cli)
		return subagentrun.Capability{BudgetNative: insp.Manifest.BudgetNative, PermissionScoping: insp.Manifest.PermissionScoping, Warns: insp.Warns}, err
	}
}

func tierOf(resolve func(ResolveModelTierRequest, ResolveModelTierOptions) (string, error)) func(subagentrun.TierRequest) (string, error) {
	return func(t subagentrun.TierRequest) (string, error) {
		return resolve(ResolveModelTierRequest{
			ProfilePath:            t.ProfilePath,
			Cycle:                  t.Cycle,
			ProjectRoot:            t.ProjectRoot,
			WorktreePath:           t.WorktreePath,
			ModelTierHint:          t.ModelTierHint,
			AuditorTierOverride:    t.AuditorTierOverride,
			DiffComplexityDisabled: t.DiffComplexityDisabled,
		}, ResolveModelTierOptions{})
	}
}

// adapterOf is the bridge exec port: the gobridge-backed adapter carrying the
// root's Center when no ExecAdapter seam was supplied (production), else the
// supplied func over the vestigial adapter path and the rendered env (the
// by-name binders' shape).
func adapterOf(opts RunOptions) subagentrun.Adapter {
	if opts.ExecAdapter == nil {
		return bridgeAdapter{signals: opts.Signals}
	}
	return subagentrun.AdapterFunc(func(ctx context.Context, e subagentrun.AdapterEnv) (int, error) {
		return opts.ExecAdapter(ctx, e.AdapterPath, e.Map())
	})
}

// capabilityTier maps Manifest support flags to the v8.51.0 quality_tier
// label used by ledger entries (the fan-out dispatcher's spelling).
func capabilityTier(m capability.Manifest) string {
	return subagentrun.QualityTier(m.BudgetNative, m.PermissionScoping)
}

// generateRunToken mints the 16-hex challenge token; a nil rng is
// crypto/rand (the fan-out dispatcher's spelling).
func generateRunToken(rng func([]byte) (int, error)) (string, error) {
	if rng == nil {
		rng = rand.Read
	}
	return subagentrun.MintToken(rng)
}

// fillRunDefaults wires the production seams for every nil option; ExecAdapter
// stays nil so adapterOf builds the Center-carrying bridge adapter.
func fillRunDefaults(opts *RunOptions) {
	if opts.ReadProfile == nil {
		opts.ReadProfile = defaultReadProfile
	}
	if opts.ResolveLLM == nil {
		opts.ResolveLLM = defaultResolveLLM
	}
	if opts.InspectCapability == nil {
		opts.InspectCapability = capability.Inspect
	}
	if opts.ResolveModelTier == nil {
		opts.ResolveModelTier = ResolveModelTier
	}
	if opts.AdapterExists == nil {
		opts.AdapterExists = driverExists
	}
	if opts.WriteFile == nil {
		opts.WriteFile = os.WriteFile
	}
	if opts.GitState == nil {
		opts.GitState = defaultGitState
	}
	if opts.StatMTime == nil {
		opts.StatMTime = defaultStatMTime
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if opts.HashFile == nil {
		opts.HashFile = defaultHashFile
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Rand == nil {
		opts.Rand = rand.Read
	}
}
