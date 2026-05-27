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

// PhaseAdvisor is the bridge-backed DynamicLLM brain. It satisfies two router
// ports: router.Proposer (Propose — the per-transition "insert this optional
// phase?" advice) and router.Planner (Plan — the upfront whole-cycle run/skip
// plan, ADR-0024 §2). Both ask an LLM via the core.Bridge port given the
// objective digest. All output is ADVISORY: the pure router.Route() clamp pass
// re-validates it against the kernel floor (mandatory spine, TDD-pin,
// ship-needs-real-audit), so a hallucinated or malformed proposal can never
// weaken the ship guarantee. Any failure is returned as an error and the caller
// degrades cleanly to the deterministic static path — "model proposes, kernel
// disposes", fail-safe to the floor.
type PhaseAdvisor struct {
	bridge  Bridge
	cli     string
	model   string
	profile string // when non-empty, used verbatim; else derived from RouteInput.ProjectRoot
}

// PhaseAdvisorOption customizes a PhaseAdvisor.
type PhaseAdvisorOption func(*PhaseAdvisor)

// WithProposerCLI overrides the CLI the proposer dispatches to.
func WithProposerCLI(cli string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if cli != "" {
			p.cli = cli
		}
	}
}

// WithProposerModel overrides the model tier the proposer requests.
func WithProposerModel(model string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if model != "" {
			p.model = model
		}
	}
}

// NewPhaseAdvisor builds a proposer over the given bridge. Defaults to a
// fast/cheap model on the tmux Claude driver — routing is a lightweight
// read-only judgment, not heavy generation.
func NewPhaseAdvisor(bridge Bridge, opts ...PhaseAdvisorOption) *PhaseAdvisor {
	p := &PhaseAdvisor{bridge: bridge, cli: "claude-tmux", model: "haiku"}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Propose implements router.Proposer.
func (p *PhaseAdvisor) Propose(in router.RouteInput) (*router.Proposal, error) {
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

// Plan implements router.Planner: it asks the LLM for a whole-cycle run/skip
// plan (ADR-0024 §2 hybrid cadence — the cheap, coherent upfront decision). The
// returned plan is ADVISORY; the kernel clamp re-validates it against the floor.
// Mirrors Propose's wiring but writes phase-plan.json and parses a JSON array.
// Any failure returns an error so the caller degrades to the static path.
func (p *PhaseAdvisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	if p.bridge == nil {
		return nil, fmt.Errorf("phase advisor: nil bridge")
	}
	if in.Workspace == "" {
		return nil, fmt.Errorf("phase advisor: empty workspace")
	}
	profile := p.profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "router.json")
	}
	resp, err := p.bridge.Launch(context.Background(), BridgeRequest{
		CLI:          p.cli,
		Profile:      profile,
		Model:        p.model,
		Prompt:       buildPlanPrompt(in),
		Workspace:    in.Workspace,
		ArtifactPath: filepath.Join(in.Workspace, "phase-plan.json"),
		Agent:        "router",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("phase advisor: bridge launch: %w", err)
	}
	plan, err := parsePhasePlan(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("phase advisor: %w", err)
	}
	return plan, nil
}

// buildRoutingPrompt renders the per-transition routing context into a compact,
// deterministic prompt. It lists the just-completed phase, the digested
// signals, the optional phases still available with their declarative triggers,
// and the non-bypassable kernel rules — then asks for a strict-JSON proposal.
func buildRoutingPrompt(in router.RouteInput) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop ROUTER. The model proposes; the kernel disposes.\n")
	b.WriteString("Given the objective signals of the phases run so far, propose which phase should run next ")
	b.WriteString("and which optional phases to insert. Your proposal is ADVISORY and will be clamped to the ")
	b.WriteString("mandatory spine, the TDD pin, and the ship-needs-audit rule — never propose skipping those.\n\n")

	writeRoutingContext(&b, in)

	b.WriteString("\n## Respond with STRICT JSON only (no prose, no markdown fence):\n")
	b.WriteString(`{"next_phase":"<phase>","insert_phases":["<phase>",...],"justification":"<one sentence>"}`)
	b.WriteString("\n")
	return b.String()
}

// buildPlanPrompt renders the WHOLE-CYCLE planning context (ADR-0024 §2): the
// same objective digest + rubric as buildRoutingPrompt, but it asks the advisor
// to decide run/skip for EVERY phase of the cycle in one coherent pass, as a
// strict-JSON array. The plan is advisory — the kernel clamp re-validates it.
func buildPlanPrompt(in router.RouteInput) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop PHASE ADVISOR. The model proposes; the kernel disposes.\n")
	b.WriteString("From the objective signals below, decide which phases should RUN this cycle and which to SKIP, ")
	b.WriteString("with a one-sentence justification per phase. Your plan is ADVISORY and will be clamped to the ")
	b.WriteString("integrity floor (ship requires a real PASS audit bound to the built tree) — a plan that reaches ")
	b.WriteString("ship without audit is rejected by the kernel.\n\n")

	writeRoutingContext(&b, in)

	b.WriteString("\n## Respond with STRICT JSON only (a bare array, no prose, no markdown fence):\n")
	b.WriteString(`[{"phase":"<phase>","run":true,"justification":"<one sentence>"},...]`)
	b.WriteString("\n")
	return b.String()
}

// writeRoutingContext writes the shared, deterministic decision context — cycle
// header, digested objective signals, available optional phases, and the
// decision rubric — consumed by both the per-transition prompt and the
// whole-cycle plan prompt. Deterministic string ⇒ prompt-prefix cache friendly.
func writeRoutingContext(b *strings.Builder, in router.RouteInput) {
	fmt.Fprintf(b, "## Cycle\n- cycle: %d\n- just_completed: %s\n- last_verdict: %s\n", in.Cycle, in.Current, in.Verdict)
	fmt.Fprintf(b, "- completed_phases: %s\n", strings.Join(in.Completed, ", "))
	fmt.Fprintf(b, "- mandatory_spine: %s\n", strings.Join(in.Cfg.Mandatory, ", "))
	fmt.Fprintf(b, "- budget_remaining_usd: %.2f\n- max_optional_insertions: %d\n\n", in.BudgetRemaining, in.Cfg.MaxInsertions)

	b.WriteString("## Objective signals (digested from handoff artifacts)\n")
	writeSignals(b, in.Signals)

	if len(in.Cfg.Triggers) > 0 {
		b.WriteString("\n## Optional phases available (insert only on objective signal)\n")
		names := make([]string, 0, len(in.Cfg.Triggers))
		for name := range in.Cfg.Triggers {
			names = append(names, name)
		}
		sort.Strings(names) // deterministic prompt ⇒ prompt-prefix cache friendly
		for _, name := range names {
			fmt.Fprintf(b, "- %s\n", name)
		}
	}

	// Decision rubric — renders skills/adversarial-testing/SKILL.md §7 inline so
	// the advisor reasons from the same objective-signal table the kernel uses.
	b.WriteString("\n## Decision rubric (justify each optional phase by an objective signal)\n")
	b.WriteString("- scout.carryover_count >= 3 → skip scout (work already queued)\n")
	b.WriteString("- scout.item_count == 0 → end cycle early (no-ship is legitimate)\n")
	b.WriteString("- build.files_touched >= 10 OR build.diff_loc >= 500 → insert plan-review\n")
	b.WriteString("- build.acs_red >= 1 OR build.severity_max >= HIGH → insert tester\n")
	b.WriteString("- audit.verdict == FAIL OR audit.confidence < 0.85 → insert retrospective\n")
	b.WriteString("- cycle_size == trivial → skip tdd (conditional-mandatory exemption)\n")
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")
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

// parsePhasePlan extracts the strict-JSON whole-cycle plan from the LLM stdout.
// The wire format is a bare array of {phase, run, justification}; like
// parseProposal it tolerates a surrounding ```json fence or leading/trailing
// prose by slicing from the first '[' to the last ']'. An empty or unparseable
// body is an error (caller degrades to the deterministic static plan).
func parsePhasePlan(stdout string) (*router.PhasePlan, error) {
	start := strings.IndexByte(stdout, '[')
	end := strings.LastIndexByte(stdout, ']')
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON array in plan output")
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &entries); err != nil {
		return nil, fmt.Errorf("parse phase plan: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty phase plan")
	}
	return &router.PhasePlan{Entries: entries}, nil
}

// compile-time assertions that PhaseAdvisor satisfies both router ports.
var (
	_ router.Proposer = (*PhaseAdvisor)(nil)
	_ router.Planner  = (*PhaseAdvisor)(nil)
)
