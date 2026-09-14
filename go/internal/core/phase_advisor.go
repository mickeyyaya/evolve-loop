package core

// phase_advisor.go — unit 04 (ADR-0103, design decomposition/04-advisor.md):
// core's seam onto the phase advisor. The brain lives in internal/core/advisor
// behind a leaf-owned Launcher port; this file keeps the host struct the
// composition root builds with its four options, projects the core Bridge
// onto that port ONCE, and holds the facades every old caller keeps its
// spelling through — the router ports, replay, the span/identity aliases,
// resume's plan parser, the judge's and the adjudicator's span scanner, the
// task-recall cap, the failure digest's atomic writer, the orchestrator's
// bench projection and the git reader the seam injects — plus the test
// facades the ACS-named and protected core tests call by name.

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
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// PhaseAdvisor is the bridge-backed DynamicLLM brain. It satisfies two router
// ports: router.Proposer (Propose — the per-transition "insert this optional
// phase?" advice) and router.Planner (Plan — the upfront whole-cycle run/skip
// plan, ADR-0024 §2). Both ask an LLM via the core.Bridge port given the
// objective digest. All output is ADVISORY: the pure router.Route() clamp pass
// re-validates it against the kernel floor (mandatory spine, TDD-pin,
// ship-needs-real-audit), so a hallucinated or malformed proposal can never
// weaken the ship guarantee. Any failure is returned as an error and the caller
// degrades cleanly to the deterministic static path — "model proposes, kernel
// disposes", fail-safe to the floor. Since ADR-0103 unit 04 it is the host of
// the advisor leaf: the options mutate the identity, the depth guard and the
// Center before the ONE construction.
type PhaseAdvisor struct {
	bridge   Bridge
	identity AgentIdentity // ADR-0052 WS1-S1: the shared dispatch identity (cli/model/profile/persona/label)
	// checkDepth is an injectable depth guard (defense-in-depth recursion check).
	// nil = skip the check. Wire AdvisorDepthExceeded via WithDepthCheck for production.
	checkDepth func(env map[string]string) bool
	signals    *signalcenter.Center // set by WithAdvisorSignals at the composition root
	brainOnce  sync.Once            // guards the ONE construction of brain (a literal may be first-used concurrently)
	brain      *advisor.Advisor     // the unit-04 leaf, built once
}

// PhaseAdvisorOption customizes a PhaseAdvisor.
type PhaseAdvisorOption func(*PhaseAdvisor)

// WithProposerCLI overrides the CLI the advisor dispatches to. The composition
// root resolves this from the router profile + EVOLVE_ROUTER_CLI (same path as
// phases), so the brain is configurable to any LLM CLI (claude/codex/agy).
func WithProposerCLI(cli string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if cli != "" {
			p.identity.CLI = cli
		}
	}
}

// WithProposerModel overrides the model tier the advisor requests. Resolved by
// the composition root from the router profile + EVOLVE_ROUTER_MODEL.
func WithProposerModel(model string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if model != "" {
			p.identity.Model = model
		}
	}
}

// WithDepthCheck injects the recursion-depth guard (defense-in-depth, ADR-0052 §4.3).
// When fn returns true for the dispatch env, the launch errors before the bridge.
// Pass AdvisorDepthExceeded for production behavior; nil (the zero-field default) skips the check.
func WithDepthCheck(fn func(env map[string]string) bool) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		p.checkDepth = fn
	}
}

// WithPersona injects the advisor's persona body (agents/evolve-router.md),
// making the brain defined identically to every phase agent (persona + profile +
// artifact). Empty ⇒ the legacy inline framing is used as a fail-safe.
func WithPersona(body string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if body != "" {
			p.identity.Persona = body
		}
	}
}

// WithAdvisorSignals hands the root's Signal Center to the advisor (ADR-0103
// unit 04): the brain reports its faults as advisor.warning through it. The
// composition root builds the Center before the advisor in the same function,
// so this is honest construction-time DI — the ONE declared root spelling change.
func WithAdvisorSignals(c *signalcenter.Center) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) { p.signals = c }
}

// NewPhaseAdvisor builds the routing brain over the given bridge. The cli/model
// FALLBACK is deep (opus) on the tmux Claude driver — composing the cycle and
// inventing phases is deep-reasoning work, not lightweight routing — but the
// composition root normally overrides both from the router profile + env so the
// brain is configurable to any CLI/model.
func NewPhaseAdvisor(bridge Bridge, opts ...PhaseAdvisorOption) *PhaseAdvisor {
	p := &PhaseAdvisor{
		bridge:   bridge,
		identity: AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"},
	}
	for _, o := range opts {
		o(p)
	}
	p.advisor() // eager, through the same once-guarded accessor
	return p
}

// advisor returns the unit-04 brain, built exactly once — eagerly by
// NewPhaseAdvisor, on first use for a PhaseAdvisor assembled as a literal —
// under a sync.Once, so the lazy cache is safe by construction rather than by
// caller discipline (a bare nil-check is a read-check-write race).
func (p *PhaseAdvisor) advisor() *advisor.Advisor {
	p.brainOnce.Do(func() { p.brain = p.wiredAdvisor() })
	return p.brain
}

// wiredAdvisor is the ONE construction of the leaf
// (TestPhaseAdvisor_OneConstructionSite): the bridge behind the Launcher port
// (a nil bridge stays nil so the legacy "nil bridge" fail-safe survives), the
// atomic capture writer the ledger's hash-bind relies on, the depth guard, the
// git reader and the Center through an accessor read live.
func (p *PhaseAdvisor) wiredAdvisor() *advisor.Advisor {
	return advisor.New(launcherOf(p.bridge), p.identity, writeArtifactAtomically,
		advisor.WithDepthCheck(p.checkDepth),
		advisor.WithRecentFiles(recentlyChangedFiles),
		advisor.WithSignals(func() *signalcenter.Center { return p.signals }))
}

// launcherOf adapts a Bridge to the leaf's port; nil stays nil.
func launcherOf(b Bridge) advisor.Launcher {
	if b == nil {
		return nil
	}
	return bridgeLauncher{b}
}

// bridgeLauncher is the Adapter from the leaf-owned Launcher port onto
// core.Bridge — the one place the fourteen launch fields meet the bridge's
// request.
type bridgeLauncher struct{ b Bridge }

func (l bridgeLauncher) Launch(ctx context.Context, req advisor.LaunchRequest) (advisor.LaunchResponse, error) {
	resp, err := l.b.Launch(ctx, bridgeRequestOf(req))
	return launchResponseOf(resp), err
}

// bridgeRequestOf is the ONE projection of the leaf's request onto the bridge's.
func bridgeRequestOf(r advisor.LaunchRequest) BridgeRequest {
	return BridgeRequest{
		// dispatch identity
		CLI:     r.CLI,
		Profile: r.Profile,
		Model:   r.Model,
		Skills:  r.Skills,
		// the prompt and the roots
		Prompt:      r.Prompt,
		Workspace:   r.Workspace,
		Worktree:    r.Worktree,
		ProjectRoot: r.ProjectRoot,
		// the deliverable contract
		ArtifactPath: r.ArtifactPath,
		Completion:   r.Completion,
		Agent:        r.Agent,
		Contract:     r.Contract,
		// the cycle
		Cycle: r.Cycle,
		Env:   r.Env,
	}
}

// launchResponseOf projects the four response fields the advisor reads.
func launchResponseOf(r BridgeResponse) advisor.LaunchResponse {
	return advisor.LaunchResponse{ExitCode: r.ExitCode, Stdout: r.Stdout, DurationMS: r.DurationMS, Tokens: r.Tokens}
}

// Propose implements router.Proposer.
func (p *PhaseAdvisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	return p.advisor().Propose(in)
}

// Plan implements router.Planner — the upfront whole-cycle run/skip plan.
func (p *PhaseAdvisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.advisor().Plan(in)
}

// RePlan is the post-scout re-plan (ADR-0052 WS1-S3), called in shadow every
// cycle by the orchestrator under the cfg.RouterReplan dial.
func (p *PhaseAdvisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.advisor().RePlan(in)
}

// compile-time assertions that PhaseAdvisor satisfies the router ports and
// the orchestrator's re-plan seam.
var (
	_ router.Proposer = (*PhaseAdvisor)(nil)
	_ router.Planner  = (*PhaseAdvisor)(nil)
	_ rePlanner       = (*PhaseAdvisor)(nil)
)

// AdvisorSpan is the OTel-GenAI decision span (ADR-0052 WS3-S3) — the leaf's
// Span, kept under its core spelling for the routing explain command.
type AdvisorSpan = advisor.Span

// ReplayPlanFromResponse reparses a captured advisor response through the
// SAME parse + integrity-floor clamp the live planning path runs (WS3-S5
// replay, the routing-eval corpus) — the leaf's, under its core spelling.
func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	return advisor.ReplayPlanFromResponse(raw, in, floor)
}

// parsePhasePlan is resume's re-parse of routing-plan.json: the leaf's parser
// with the rejected mints dropped (they were reported at decision time).
func parsePhasePlan(stdout string) (*router.PhasePlan, error) {
	parsed, err := advisor.ParsePhasePlan(stdout)
	if err != nil {
		return nil, err
	}
	return parsed.Plan, nil
}

// lastBalancedSpan is the plan judge's and the retry adjudicator's
// string-literal-aware span scanner — the leaf's.
func lastBalancedSpan(s string, open, close byte) (start, end int, ok bool) {
	return advisor.LastBalancedSpan(s, open, close)
}

// truncateGoal is the plan judge's goal cap — the advisor's.
func truncateGoal(s string) string { return advisor.TruncateGoal(s) }

// maxGoalTextChars is the task-recall digest's bound — the advisor's.
const maxGoalTextChars = advisor.MaxGoalTextRunes

// composePlanPrompt renders the whole-cycle plan prompt through the brain.
//
// Deprecated: test facade — the ACS-named tier-elicitation and replay tests
// (acs/cycle463, cycle476) and the resident catalog/recon/mint tests call it;
// production goes through Plan/RePlan.
func (p *PhaseAdvisor) composePlanPrompt(in router.RouteInput, artifactFile string) string {
	return p.advisor().ComposePlanPrompt(in, artifactFile)
}

// sanitizeAdvisorTier confines an advisor-emitted tier to the canonical set.
//
// Deprecated: test facade — acs/cycle517 names TestSanitizeAdvisorTier.
func sanitizeAdvisorTier(tier string) string { return advisor.SanitizeTier(tier) }

// writeCatalog renders the SELECT menu with no on-demand index.
//
// Deprecated: test facade — acs/cycle420 names the TestWriteCatalog_* pins.
func writeCatalog(b *strings.Builder, cards []router.PhaseCard) { advisor.WriteCatalog(b, cards) }

// writeCatalogWithOnDemand renders the SELECT menu and the on-demand index.
//
// Deprecated: test facade — the resident on-demand wiring tests build their
// cards from the orchestrator's catalog.
func writeCatalogWithOnDemand(b *strings.Builder, cards []router.PhaseCard, onDemand []string) {
	advisor.WriteCatalogWithOnDemand(b, cards, onDemand)
}

// writeRoutingContext renders the shared decision context.
//
// Deprecated: test facade — the judgment-lesson tests (acs/cycle1532/1549)
// and the resident unavailable-phase pin call it.
func writeRoutingContext(b *strings.Builder, in router.RouteInput) {
	advisor.WriteRoutingContext(b, in)
}

// writeCarryoverTodos renders the carryover section.
//
// Deprecated: test facade — acs/cycle488 and cycle507 name the length and
// order pins.
func writeCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	advisor.WriteCarryoverTodos(b, todos)
}

// The prompt bounds the by-name tests read — projections of the leaf's.
const (
	maxCarryoverTodosInPrompt = advisor.MaxCarryoverTodosInPrompt
	maxEnrichedCatalogCards   = advisor.MaxEnrichedCatalogCards
)

// mintConfigsFrom reconstructs the minted phase configs, the recursion
// guard's drops discarded.
//
// Deprecated: test facade — the protected phase_advisor_guard_test.go and the
// ADR-0052 seams guard name it.
func mintConfigsFrom(entries []router.PhasePlanEntry) []phaseconfig.PhaseConfig {
	mints, _ := advisor.MintConfigsFrom(entries)
	return mints
}

// reservedAdvisorMintReason is the recursion guard's reason for a name.
//
// Deprecated: test facade — the protected phase_advisor_guard_test.go names it.
func reservedAdvisorMintReason(name string) string { return advisor.ReservedMintReason(name) }

// writeArtifactAtomically writes data via a temp file + rename, so a capture
// artifact on disk is always either absent or COMPLETE — never a truncated
// half-write (a crash mid-write leaves a stale .tmp, not a corrupt capture).
// This is what keeps the ledger's disk-read SHA (WS3-S2 bindArtifactSHA) equal
// to the span's in-memory SHA (WS3-S3) for the same bytes, and matches the
// repo's atomic-write convention. Single-writer per (workspace, kind) per
// cycle, so the fixed .tmp suffix cannot collide. The failure digest writes
// through it too; the seam injects it into the advisor leaf positionally.
func writeArtifactAtomically(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// benchedCLIsForRouting projects the cli-health store's ACTIVE benches into
// the advisor's environmental context, sorted by family for a deterministic
// (prompt-prefix-cache-friendly) prompt. Empty when the store is empty or
// unreadable — CLI health is advice, never a planning prerequisite.
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

// recentChangeCommits bounds the git-log window the pre-plan recon scans for
// changed files (churn signal). Small + fixed ⇒ cheap and deterministic.
const recentChangeCommits = "30"

// reconGitTimeout bounds the recon's git subprocess so a hung/huge repo can never
// stall plan composition — the recon is advisory and fails open, so a timeout
// just yields no file facts (same as any other git error).
const reconGitTimeout = 5 * time.Second

// recentlyChangedFiles returns the files touched across the last
// recentChangeCommits commits (duplicates kept — frequency = churn), or the
// git error (the advisor leaf reports it and composes without file facts);
// an empty root reads nothing. One git call; reuses gitexec. The I/O lives
// HERE (core, the I/O layer) and is injected into the leaf.
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
