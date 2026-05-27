package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// RoutingProposer is the bridge-backed router.Proposer (the DynamicLLM brain).
// It asks an LLM, via the existing core.Bridge port, which optional phases to
// insert/skip next given the objective digest. Its output is ADVISORY: the pure
// router.Route() clamp pass re-validates it against the kernel floor (mandatory
// spine, TDD-pin, ship-needs-real-audit), so a hallucinated or malformed
// proposal can never weaken the ship guarantee. Any failure is returned as an
// error and LLMProposal.Decide degrades cleanly to the deterministic
// StaticPreset — "model proposes, kernel disposes", fail-safe to the floor.
type RoutingProposer struct {
	bridge  Bridge
	cli     string
	model   string
	profile string // when non-empty, used verbatim; else derived from RouteInput.ProjectRoot
}

// RoutingProposerOption customizes a RoutingProposer.
type RoutingProposerOption func(*RoutingProposer)

// WithProposerCLI overrides the CLI the proposer dispatches to.
func WithProposerCLI(cli string) RoutingProposerOption {
	return func(p *RoutingProposer) {
		if cli != "" {
			p.cli = cli
		}
	}
}

// WithProposerModel overrides the model tier the proposer requests.
func WithProposerModel(model string) RoutingProposerOption {
	return func(p *RoutingProposer) {
		if model != "" {
			p.model = model
		}
	}
}

// NewRoutingProposer builds a proposer over the given bridge. Defaults to a
// fast/cheap model on the tmux Claude driver — routing is a lightweight
// read-only judgment, not heavy generation.
func NewRoutingProposer(bridge Bridge, opts ...RoutingProposerOption) *RoutingProposer {
	p := &RoutingProposer{bridge: bridge, cli: "claude-tmux", model: "haiku"}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Propose implements router.Proposer.
func (p *RoutingProposer) Propose(in router.RouteInput) (*router.Proposal, error) {
	if p.bridge == nil {
		return nil, fmt.Errorf("routing proposer: nil bridge")
	}
	if in.Workspace == "" {
		return nil, fmt.Errorf("routing proposer: empty workspace")
	}
	profile := p.profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "router.json")
	}
	resp, err := p.bridge.Launch(context.Background(), BridgeRequest{
		CLI:          p.cli,
		Profile:      profile,
		Model:        p.model,
		Prompt:       buildRoutingPrompt(in),
		Workspace:    in.Workspace,
		ArtifactPath: filepath.Join(in.Workspace, "routing-proposal.json"),
		Agent:        "router",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("routing proposer: bridge launch: %w", err)
	}
	prop, err := parseProposal(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("routing proposer: %w", err)
	}
	return prop, nil
}

// buildRoutingPrompt renders the objective routing context into a compact,
// deterministic prompt. It lists the just-completed phase, the digested
// signals, the optional phases still available with their declarative triggers,
// and the non-bypassable kernel rules — then asks for a strict-JSON proposal.
func buildRoutingPrompt(in router.RouteInput) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop ROUTER. The model proposes; the kernel disposes.\n")
	b.WriteString("Given the objective signals of the phases run so far, propose which phase should run next ")
	b.WriteString("and which optional phases to insert. Your proposal is ADVISORY and will be clamped to the ")
	b.WriteString("mandatory spine, the TDD pin, and the ship-needs-audit rule — never propose skipping those.\n\n")

	fmt.Fprintf(&b, "## Cycle\n- cycle: %d\n- just_completed: %s\n- last_verdict: %s\n", in.Cycle, in.Current, in.Verdict)
	fmt.Fprintf(&b, "- completed_phases: %s\n", strings.Join(in.Completed, ", "))
	fmt.Fprintf(&b, "- mandatory_spine: %s\n", strings.Join(in.Cfg.Mandatory, ", "))
	fmt.Fprintf(&b, "- budget_remaining_usd: %.2f\n- max_optional_insertions: %d\n\n", in.BudgetRemaining, in.Cfg.MaxInsertions)

	b.WriteString("## Objective signals (digested from handoff artifacts)\n")
	writeSignals(&b, in.Signals)

	if len(in.Cfg.Triggers) > 0 {
		b.WriteString("\n## Optional phases available (insert only on objective signal)\n")
		names := make([]string, 0, len(in.Cfg.Triggers))
		for name := range in.Cfg.Triggers {
			names = append(names, name)
		}
		sort.Strings(names) // deterministic prompt ⇒ prompt-prefix cache friendly
		for _, name := range names {
			fmt.Fprintf(&b, "- %s\n", name)
		}
	}

	// Decision rubric — renders skills/adversarial-testing/SKILL.md §7 inline so
	// the advisor reasons from the same objective-signal table the kernel uses.
	// Deterministic string ⇒ prompt-prefix cache friendly.
	b.WriteString("\n## Decision rubric (justify each optional phase by an objective signal)\n")
	b.WriteString("- scout.carryover_count >= 3 → skip scout (work already queued)\n")
	b.WriteString("- scout.item_count == 0 → end cycle early (no-ship is legitimate)\n")
	b.WriteString("- build.files_touched >= 10 OR build.diff_loc >= 500 → insert plan-review\n")
	b.WriteString("- build.acs_red >= 1 OR build.severity_max >= HIGH → insert tester\n")
	b.WriteString("- audit.verdict == FAIL OR audit.confidence < 0.85 → insert retrospective\n")
	b.WriteString("- cycle_size == trivial → skip tdd (conditional-mandatory exemption)\n")
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")

	b.WriteString("\n## Respond with STRICT JSON only (no prose, no markdown fence):\n")
	b.WriteString(`{"next_phase":"<phase>","insert_phases":["<phase>",...],"justification":"<one sentence>"}`)
	b.WriteString("\n")
	return b.String()
}

func writeSignals(b *strings.Builder, s router.RoutingSignals) {
	if s.Scout.Present {
		fmt.Fprintf(b, "- scout: cycle_size_estimate=%s item_count=%d carryover=%d backlog=%d\n",
			s.Scout.CycleSizeEstimate, s.Scout.ItemCount, s.Scout.CarryoverCount, s.Scout.BacklogSize)
	}
	if s.Triage.Present {
		fmt.Fprintf(b, "- triage: cycle_size=%s phase_skip=%s\n", s.Triage.CycleSize, strings.Join(s.Triage.PhaseSkip, ","))
	}
	if s.Build.Present {
		fmt.Fprintf(b, "- build: verdict=%s acs_green=%d acs_red=%d acs_regression=%d severity_max=%s files_touched=%d diff_loc=%d\n",
			s.Build.Verdict, s.Build.ACSGreen, s.Build.ACSRed, s.Build.ACSRegression, s.Build.SeverityMax, s.Build.FilesTouched, s.Build.DiffLOC)
	}
	if s.Audit.Present {
		fmt.Fprintf(b, "- audit: verdict=%s confidence=%.2f red_count=%d\n", s.Audit.Verdict, s.Audit.Confidence, s.Audit.RedCount)
	}
}

// parseProposal extracts the strict-JSON proposal from the LLM stdout. It is
// tolerant of a surrounding ```json fence or leading/trailing prose: it slices
// from the first '{' to the last '}'. An empty or unparseable body is an error
// (caller degrades to static).
func parseProposal(stdout string) (*router.Proposal, error) {
	start := strings.IndexByte(stdout, '{')
	end := strings.LastIndexByte(stdout, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON object in proposer output")
	}
	var prop router.Proposal
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &prop); err != nil {
		return nil, fmt.Errorf("parse proposal: %w", err)
	}
	if prop.NextPhase == "" && len(prop.InsertPhases) == 0 {
		return nil, fmt.Errorf("empty proposal")
	}
	return &prop, nil
}

// compile-time assertion that RoutingProposer satisfies the router port.
var _ router.Proposer = (*RoutingProposer)(nil)
