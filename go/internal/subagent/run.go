package subagent

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/resolvellm"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

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

	PromptReader io.Reader

	ModelTierHint          string
	AuditorTierOverride    string
	DiffComplexityDisabled bool
	AdversarialAudit       bool
	LegacyAgentDispatch    bool
	DispatchDepth          int
	ChallengeTokenOverride string
}

var ErrInProcessDispatchBanned = subagentrun.ErrInProcessDispatchBanned

type RunOptions struct {
	ReadProfile       func(path string) (string, error)
	ResolveLLM        func(agent string) (resolvellm.Result, error)
	InspectCapability func(adaptersDir, cli string) (capability.Inspection, error)
	ResolveModelTier  func(req ResolveModelTierRequest, opts ResolveModelTierOptions) (string, error)
	AdapterExists     func(cli string) bool
	ExecAdapter       func(ctx context.Context, adapterPath string, env map[string]string) (exitCode int, err error)
	// Deprecated: dead seam, retired with the next binder edit (unit 16 follow-up 16-4).
	WriteFile func(path string, data []byte, mode os.FileMode) error
	GitState  func(ctx context.Context, projectRoot string) (head, treeDiff string, err error)
	StatMTime func(path string) (time.Time, error)
	ReadFile  func(path string) ([]byte, error)
	HashFile  func(path string) (string, error)
	Now       func() time.Time
	Rand      func([]byte) (int, error)
	Signals   *signalcenter.Center
}

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
	Stderr         string
}

var nonRegistryRoles = []string{
	"inspirer", "evaluator", "plan-reviewer", "memo", "tester",
}

var agentRoles = buildAgentRoles()

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

var agentRolePattern = regexp.MustCompile(`^(` + strings.Join(agentRoles, "|") + `)$`)

func Run(ctx context.Context, req RunRequest, opts RunOptions) (RunResult, error) {
	fillRunDefaults(&opts)
	out, err := wiredDispatcher(opts).Dispatch(ctx, requestOf(req))
	return resultOf(out), err
}

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

func resultOf(out subagentrun.Outcome) RunResult {
	return RunResult{
		Verdict: out.Verdict, CLI: out.CLI, Model: out.Model,
		ArtifactPath: out.ArtifactPath, ArtifactSHA256: out.ArtifactSHA256, ChallengeToken: out.ChallengeToken,
		ExitCode: out.ExitCode, DurationMS: out.DurationMS, Warns: out.Warns,
	}
}

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
		if errors.Is(err, cliroute.ErrRefused) {
			err = fmt.Errorf("%w: %w", subagentrun.ErrRouteRefused, err)
		}
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

func adapterOf(opts RunOptions) subagentrun.Adapter {
	if opts.ExecAdapter == nil {
		return bridgeAdapter{signals: opts.Signals}
	}
	return subagentrun.AdapterFunc(func(ctx context.Context, e subagentrun.AdapterEnv) (int, error) {
		return opts.ExecAdapter(ctx, e.AdapterPath, e.Map())
	})
}

func capabilityTier(m capability.Manifest) string {
	return subagentrun.QualityTier(m.BudgetNative, m.PermissionScoping)
}

func generateRunToken(rng func([]byte) (int, error)) (string, error) {
	if rng == nil {
		rng = rand.Read
	}
	return subagentrun.MintToken(rng)
}

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
