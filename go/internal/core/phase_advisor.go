package core

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type PhaseAdvisor struct {
	bridge     Bridge
	identity   AgentIdentity
	checkDepth func(env map[string]string) bool
	signals    *signalcenter.Center
	brainOnce  sync.Once
	brain      *advisor.Advisor
	walk       llmroute.Plan
}

type PhaseAdvisorOption func(*PhaseAdvisor)

func WithProposerCLI(cli string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if cli != "" {
			p.identity.CLI = cli
		}
	}
}

func WithProposerModel(model string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if model != "" {
			p.identity.Model = model
		}
	}
}

func WithProposerWalk(walk llmroute.Plan) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) { p.walk = walk }
}

func WithDepthCheck(fn func(env map[string]string) bool) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		p.checkDepth = fn
	}
}

func WithPersona(body string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if body != "" {
			p.identity.Persona = body
		}
	}
}

func WithAdvisorSignals(c *signalcenter.Center) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) { p.signals = c }
}

func NewPhaseAdvisor(bridge Bridge, opts ...PhaseAdvisorOption) *PhaseAdvisor {
	p := &PhaseAdvisor{
		bridge:   bridge,
		identity: AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: routerAgentLabel},
	}
	for _, o := range opts {
		o(p)
	}
	p.advisor()
	return p
}

func (p *PhaseAdvisor) advisor() *advisor.Advisor {
	p.brainOnce.Do(func() { p.brain = p.wiredAdvisor() })
	return p.brain
}

func (p *PhaseAdvisor) wiredAdvisor() *advisor.Advisor {
	return advisor.New(launcherOf(p.bridge), p.identity, writeArtifactAtomically,
		advisor.WithDepthCheck(p.checkDepth),
		advisor.WithRecentFiles(recentlyChangedFiles),
		advisor.WithSignals(func() *signalcenter.Center { return p.signals }),
		advisor.WithRoute(p.walk))
}

func launcherOf(b Bridge) advisor.Launcher {
	if b == nil {
		return nil
	}
	return bridgeLauncher{b}
}

type bridgeLauncher struct{ b Bridge }

func (l bridgeLauncher) Launch(ctx context.Context, req advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	resp, err := LaunchWithFreshSession(ctx, l.b, bridgeRequestOf(req))
	return launchResponseOf(resp), err
}

func bridgeRequestOf(r advisor.LaunchRequest) BridgeRequest {
	return BridgeRequest{
		CLI:          r.CLI,
		Profile:      r.Profile,
		Model:        r.Model,
		Skills:       r.Skills,
		Prompt:       r.Prompt,
		Workspace:    r.Workspace,
		Worktree:     r.Worktree,
		ProjectRoot:  r.ProjectRoot,
		ArtifactPath: r.ArtifactPath,
		Completion:   CompletionContract(r.Completion),
		Agent:        r.Agent,
		Contract:     r.Contract,
		Cycle:        r.Cycle,
		Env:          r.Env,
		ChainAttempt: true,
	}
}

func launchResponseOf(r BridgeResponse) advisor.LaunchResponse {
	return advisor.LaunchResponse{ExitCode: r.ExitCode, Stdout: r.Stdout, DurationMS: r.DurationMS, Tokens: r.Tokens}
}

func (p *PhaseAdvisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	return p.advisor().Propose(in)
}

func (p *PhaseAdvisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.advisor().Plan(in)
}

func (p *PhaseAdvisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.advisor().RePlan(in)
}

var (
	_ router.Proposer = (*PhaseAdvisor)(nil)
	_ router.Planner  = (*PhaseAdvisor)(nil)
	_ rePlanner       = (*PhaseAdvisor)(nil)
)

type AdvisorSpan = advisor.Span

func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	return advisor.ReplayPlanFromResponse(raw, in, floor)
}

func parsePhasePlan(stdout string) (*router.PhasePlan, error) {
	parsed, err := advisor.ParsePhasePlan(stdout)
	if err != nil {
		return nil, err
	}
	return parsed.Plan, nil
}

func lastBalancedSpan(s string, open, close byte) (start, end int, ok bool) {
	return advisor.LastBalancedSpan(s, open, close)
}

func truncateGoal(s string) string { return advisor.TruncateGoal(s) }

const maxGoalTextChars = advisor.MaxGoalTextRunes

// Deprecated: test facade — the ACS-named tier-elicitation and replay tests
// (acs/cycle463, cycle476) and the resident catalog/recon/mint tests call it;
// production goes through Plan/RePlan.
func (p *PhaseAdvisor) composePlanPrompt(in router.RouteInput, artifactFile string) string {
	return p.advisor().ComposePlanPrompt(in, artifactFile)
}

// Deprecated: test facade — acs/cycle517 names TestSanitizeAdvisorTier.
func sanitizeAdvisorTier(tier string) string { return advisor.SanitizeTier(tier) }

// Deprecated: test facade — acs/cycle420 names the TestWriteCatalog_* pins.
func writeCatalog(b *strings.Builder, cards []router.PhaseCard) { advisor.WriteCatalog(b, cards) }

// Deprecated: test facade — the resident on-demand wiring tests build their
// cards from the orchestrator's catalog.
func writeCatalogWithOnDemand(b *strings.Builder, cards []router.PhaseCard, onDemand []string) {
	advisor.WriteCatalogWithOnDemand(b, cards, onDemand)
}

// Deprecated: test facade — the judgment-lesson tests (acs/cycle1532/1549)
// and the resident unavailable-phase pin call it.
func writeRoutingContext(b *strings.Builder, in router.RouteInput) {
	advisor.WriteRoutingContext(b, in)
}

// Deprecated: test facade — acs/cycle488 and cycle507 name the length and
// order pins.
func writeCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	advisor.WriteCarryoverTodos(b, todos)
}

const (
	maxCarryoverTodosInPrompt = advisor.MaxCarryoverTodosInPrompt
	maxEnrichedCatalogCards   = advisor.MaxEnrichedCatalogCards
)

// Deprecated: test facade — the protected phase_advisor_guard_test.go and the
// ADR-0052 seams guard name it.
func mintConfigsFrom(entries []router.PhasePlanEntry) []phaseconfig.PhaseConfig {
	mints, _ := advisor.MintConfigsFrom(entries)
	return mints
}

// Deprecated: test facade — the protected phase_advisor_guard_test.go names it.
func reservedAdvisorMintReason(name string) string { return advisor.ReservedMintReason(name) }

func writeArtifactAtomically(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func benchedCLIsForRouting(projectRoot string) []router.BenchedCLI {
	active := clihealth.NewStore(projectRoot, nil).Active()
	if len(active) == 0 {
		return nil
	}
	out := make([]router.BenchedCLI, 0, len(active))
	for _, e := range active {
		out = append(out, router.BenchedCLI{Family: e.Family, Reason: e.Reason, Until: e.BenchedUntil})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Family < out[j].Family })
	return out
}

const recentChangeCommits = "30"

const reconGitTimeout = 5 * time.Second

func recentlyChangedFiles(projectRoot string) ([]string, error) {
	if projectRoot == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), reconGitTimeout)
	defer cancel()
	out, err := gitexec.Default(projectRoot).Output(ctx, "log", "-n", recentChangeCommits, "--name-only", "--pretty=format:")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
