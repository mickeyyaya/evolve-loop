package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
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
	persona string // agents/evolve-router.md body; injected by the composition root (uniform with phase agents). Empty ⇒ legacy inline framing.
}

// PhaseAdvisorOption customizes a PhaseAdvisor.
type PhaseAdvisorOption func(*PhaseAdvisor)

// WithProposerCLI overrides the CLI the advisor dispatches to. The composition
// root resolves this from the router profile + EVOLVE_ROUTER_CLI (same path as
// phases), so the brain is configurable to any LLM CLI (claude/codex/agy).
func WithProposerCLI(cli string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if cli != "" {
			p.cli = cli
		}
	}
}

// WithProposerModel overrides the model tier the advisor requests. Resolved by
// the composition root from the router profile + EVOLVE_ROUTER_MODEL.
func WithProposerModel(model string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if model != "" {
			p.model = model
		}
	}
}

// WithPersona injects the advisor's persona body (agents/evolve-router.md),
// making the brain defined identically to every phase agent (persona + profile +
// artifact). Empty ⇒ the legacy inline framing is used as a fail-safe.
func WithPersona(body string) PhaseAdvisorOption {
	return func(p *PhaseAdvisor) {
		if body != "" {
			p.persona = body
		}
	}
}

// NewPhaseAdvisor builds the routing brain over the given bridge. The cli/model
// FALLBACK is deep (opus) on the tmux Claude driver — composing the cycle and
// inventing phases is deep-reasoning work, not lightweight routing — but the
// composition root normally overrides both from the router profile + env so the
// brain is configurable to any CLI/model.
func NewPhaseAdvisor(bridge Bridge, opts ...PhaseAdvisorOption) *PhaseAdvisor {
	p := &PhaseAdvisor{bridge: bridge, cli: "claude-tmux", model: "opus"}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Propose implements router.Proposer.
func (p *PhaseAdvisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	resp, err := p.advisorLaunch(in, "routing proposer", buildRoutingPrompt(in), "routing-proposal.json", "stdout")
	if err != nil {
		return nil, err
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
// Mirrors Propose's wiring but writes routing-plan.json and parses a JSON array.
// Any failure returns an error so the caller degrades to the static path.
func (p *PhaseAdvisor) Plan(in router.RouteInput) (*router.PhasePlan, error) {
	// The advisor's raw plan artifact (routing-plan.json) is distinct from the
	// orchestrator's clamped phase-plan.json (written by recordPhasePlan):
	// keeping them separate preserves both for forensics (advisory vs disposed).
	resp, err := p.advisorLaunch(in, "phase advisor", p.composePlanPrompt(in), "routing-plan.json", "artifact")
	if err != nil {
		return nil, err
	}
	plan, err := parsePhasePlan(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("phase advisor: %w", err)
	}
	return plan, nil
}

// composePlanPrompt builds the whole-cycle planning prompt the uniform way: the
// persona body (agents/evolve-router.md — identity, job, mint guidance, output
// contract) followed by the DYNAMIC per-cycle context (objective digest, recall
// memory, catalog, decision rubric) appended in Go, exactly as a phase appends
// its cycle context. When no persona was injected it falls back to the legacy
// fully-inline framing (buildPlanPrompt) so the advisor still functions.
func (p *PhaseAdvisor) composePlanPrompt(in router.RouteInput) string {
	if p.persona == "" {
		return buildPlanPrompt(in)
	}
	var b strings.Builder
	b.WriteString(p.persona)
	b.WriteString("\n\n---\n# This cycle\n\n")
	writeRoutingContext(&b, in)
	writeCatalog(&b, in.Catalog)
	// Instruct the ABSOLUTE artifact path — the same path advisorLaunch tells the
	// bridge to watch (filepath.Join(in.Workspace, "routing-plan.json")). A relative
	// path lands in the REPL's cwd (under claude-tmux that is NOT the workspace — it
	// varies per cycle), so the bridge never sees it and the artifact-wait times out
	// → degrade to static (the cycle-210 failure). Absolute path = lands where watched.
	fmt.Fprintf(&b, "\nNow write your whole-cycle plan as a strict JSON array to %s (no prose, no fence).\n", filepath.Join(in.Workspace, "routing-plan.json"))
	return b.String()
}

// advisorLaunch is the shared wiring for Propose and Plan: it guards the
// required fields, resolves the router profile, and launches the bridge under
// the given completion contract.
//
// Plan uses completion="artifact" (the uniform, robust contract): the brain
// WRITES routing-plan.json and the bridge reads it back into resp.Stdout — same
// as every phase writes its report. Propose still uses completion="stdout"
// (ADR-0027 REPL-idle scrollback) pending its own unification. Either way, a
// failure returns an error and the caller degrades cleanly to the static path.
func (p *PhaseAdvisor) advisorLaunch(in router.RouteInput, errPfx, prompt, artifactFile, completion string) (BridgeResponse, error) {
	if p.bridge == nil {
		return BridgeResponse{}, fmt.Errorf("%s: nil bridge", errPfx)
	}
	if in.Workspace == "" {
		return BridgeResponse{}, fmt.Errorf("%s: empty workspace", errPfx)
	}
	profile := p.profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "router.json")
	}
	resp, err := p.bridge.Launch(context.Background(), BridgeRequest{
		CLI:          p.cli,
		Profile:      profile,
		Model:        p.model,
		Prompt:       prompt,
		Workspace:    in.Workspace,
		ArtifactPath: filepath.Join(in.Workspace, artifactFile),
		Completion:   completion,
		Agent:        "router",
		Cycle:        in.Cycle,
		Env:          in.Env,
	})
	if err != nil {
		return BridgeResponse{}, fmt.Errorf("%s: bridge launch: %w", errPfx, err)
	}
	return resp, nil
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

	writeCatalog(&b, in.Catalog)

	b.WriteString("\n## Optionally MINT a new phase\n")
	b.WriteString("If an objective signal calls for work no existing phase covers, you MAY add an entry for a brand-new phase ")
	b.WriteString("by attaching a \"mint\" block. Give it a kebab-case phase name, an inline persona prompt, and a TIER ")
	b.WriteString("(fast|balanced|deep — never a raw model name). Minted phases are always optional and clamped by the kernel; ")
	b.WriteString("they can never reach ship without audit. Omit \"mint\" for existing phases.\n")

	b.WriteString("\n## Respond with STRICT JSON only (a bare array, no prose, no markdown fence):\n")
	b.WriteString(`[{"phase":"<phase>","run":true,"justification":"<one sentence>"},`)
	b.WriteString(`{"phase":"<new-phase>","run":true,"justification":"<why>","mint":{"prompt":"<persona>","tier":"balanced","cli":"claude","writes_source":false}}]`)
	b.WriteString("\n")
	return b.String()
}

// writeCatalog renders the pre-defined phases the advisor may SELECT (WS3),
// biasing toward reuse over minting: a selectable phase already has a tuned
// persona + profile, so minting should be the exception (YAGNI for new phases).
// Deterministic order (catalog order) ⇒ prompt-prefix-cache friendly. Emits
// nothing when the catalog is empty (legacy built-in-only path).
func writeCatalog(b *strings.Builder, cards []router.PhaseCard) {
	if len(cards) == 0 {
		return
	}
	b.WriteString("\n## Pre-defined phases you may SELECT (prefer these over minting)\n")
	b.WriteString("Each already exists with a tuned persona + profile. SELECT one by naming it in your plan ")
	b.WriteString("(no \"mint\" block). Only MINT a new phase when none of these fit the work.\n")
	for _, c := range cards {
		ws := ""
		if c.WritesSource {
			ws = ", writes-source"
		}
		fmt.Fprintf(b, "- %s [%s%s]\n", c.Name, c.Role, ws)
	}
}

// maxGoalTextChars bounds the goal text rendered into the advisor prompt so an
// oversized operator-pasted goal cannot crowd the catalog + rubric out of the
// context window.
const maxGoalTextChars = 4000

// truncateGoal trims surrounding whitespace and caps the goal at maxGoalTextChars
// (rune-safe), marking truncation. Empty/whitespace-only ⇒ "" (no Goal section).
func truncateGoal(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxGoalTextChars {
		return s
	}
	return string(r[:maxGoalTextChars]) + " …[truncated]"
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

	// The goal text (when threaded) is the brain's primary input for composing the
	// cycle: it lets the advisor judge whether the work is novel/cross-cutting
	// enough to warrant a design phase or a minted phase, instead of planning blind.
	// Capped so an oversized operator-pasted goal cannot push the catalog/rubric out
	// of the context window. Placed before the per-cycle signals: the goal is stable
	// across a run's cycles, so a stable section stays ahead of the volatile ones.
	if g := truncateGoal(in.GoalText); g != "" {
		fmt.Fprintf(b, "## Goal\n%s\n\n", g)
	}

	b.WriteString("## Objective signals (digested from handoff artifacts)\n")
	writeSignals(b, in.Signals)

	writeCarryoverTodos(b, in.CarryoverTodos)

	writeRecallMemory(b, in)

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
	b.WriteString("- scout.cycle_size == large OR a novel/cross-cutting goal → insert architecture-design (a design pass before tdd/build)\n")
	b.WriteString("- build.files_touched >= 10 OR build.diff_loc >= 500 → insert plan-review\n")
	b.WriteString("- build.acs_red >= 1 OR build.severity_max >= HIGH → insert tester\n")
	b.WriteString("- audit.verdict == FAIL OR audit.confidence < 0.85 → insert retrospective\n")
	b.WriteString("- cycle_size == trivial → skip tdd (conditional-mandatory exemption)\n")
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")
}

// writeRecallMemory renders the WS2 recall section — the most recent failure's
// short reason and the prior lessons that match it — so the advisor plans WITH
// the benefit of what went wrong before (Reflexion-style recall). Both fields
// are pre-computed by the orchestrator (KB lookup is its I/O, not the advisor's),
// so this stays a pure deterministic render. Emits nothing when there is neither
// a reason nor a lesson, keeping the prompt prefix stable for the no-history case.
func writeRecallMemory(b *strings.Builder, in router.RouteInput) {
	if in.LastReason == "" && len(in.Lessons) == 0 {
		return
	}
	b.WriteString("\n## Recall memory (learn from prior cycles — do not repeat these)\n")
	if in.LastReason != "" {
		fmt.Fprintf(b, "- why the last cycle failed: %s\n", in.LastReason)
	}
	for _, lesson := range in.Lessons {
		fmt.Fprintf(b, "- lesson: %s\n", lesson)
	}
}

const maxCarryoverTodosInPrompt = 20

func writeCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	if len(todos) == 0 {
		return
	}
	b.WriteString("\n## Carryover todos from previous cycles (consider when selecting phases)\n")
	limit := len(todos)
	if limit > maxCarryoverTodosInPrompt {
		limit = maxCarryoverTodosInPrompt
	}
	for i := 0; i < limit; i++ {
		t := todos[i]
		fmt.Fprintf(b, "- [%s] %s: %s (first_seen_cycle=%d, cycles_unpicked=%d)\n",
			t.Priority, t.ID, t.Action, t.FirstSeenCycle, t.CyclesUnpicked)
	}
	if len(todos) > limit {
		fmt.Fprintf(b, "- ... %d more carryover todo(s) omitted from prompt\n", len(todos)-limit)
	}
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

// parseProposal extracts the strict-JSON proposal from the LLM stdout. Under
// the ADR-0027 stdout contract the "stdout" is the captured REPL scrollback,
// which echoes the PROMPT — and the prompt carries a JSON example. A naive
// first-'{'/last-'}' slice would span the example through the real answer, so
// we take the LAST balanced object (the agent's reply is last). Tolerant of a
// ```json fence / surrounding prose. Empty/unparseable → error (caller
// degrades to static).
func parseProposal(stdout string) (*router.Proposal, error) {
	start, end, ok := lastBalancedSpan(stdout, '{', '}')
	if !ok {
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
// parseProposal it takes the LAST balanced array so the prompt's echoed JSON
// example (present in the captured scrollback under the ADR-0027 stdout
// contract) is not mistaken for the answer. An empty or unparseable body is an
// error (caller degrades to the deterministic static plan).
func parsePhasePlan(stdout string) (*router.PhasePlan, error) {
	start, end, ok := lastBalancedSpan(stdout, '[', ']')
	if !ok {
		return nil, fmt.Errorf("no JSON array in plan output")
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal([]byte(stdout[start:end+1]), &entries); err != nil {
		return nil, fmt.Errorf("parse phase plan: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty phase plan")
	}
	return &router.PhasePlan{Entries: entries, MintPhases: mintConfigsFrom(entries)}, nil
}

// mintConfigsFrom reconstructs a phaseconfig.PhaseConfig for every entry that
// carries a Mint block. The entry's Phase becomes the phase name (and default
// agent/profile key); the MintSpec supplies the persona + dispatch knobs. The
// registrar later forces Optional + clamps the tier/cli, so this mapping does
// the minimum: name + inline prompt + tier + cli + writes_source.
//
// A mint entry is collected regardless of its Run flag: REGISTRATION (wiring the
// phase into runners/catalog/routing) is distinct from DISPATCH (whether it runs
// this cycle, which the entry's Run flag governs via the routing loop). A
// run:false mint thus reserves the phase without executing it. Returns nil (the
// common no-op path) when no entry mints.
func mintConfigsFrom(entries []router.PhasePlanEntry) []phaseconfig.PhaseConfig {
	var out []phaseconfig.PhaseConfig
	for _, e := range entries {
		if e.Mint == nil {
			continue
		}
		out = append(out, phaseconfig.PhaseConfig{
			PhaseSpec: phasespec.PhaseSpec{Name: e.Phase, WritesSource: e.Mint.WritesSource},
			Dispatch:  phaseconfig.Dispatch{CLI: e.Mint.CLI, ModelTierDefault: e.Mint.Tier},
			Prompt:    e.Mint.Prompt,
		})
	}
	return out
}

// lastBalancedSpan finds the LAST top-level balanced span delimited by open/
// close in s, returning [start, end] inclusive indices. It forward-scans while
// tracking JSON string-literal context (with backslash escapes), so a literal
// delimiter inside a "justification" value (e.g. `}` or `]`) is not miscounted.
// It records every top-level span and returns the last, so the agent's reply is
// extracted even when the scrollback also contains an earlier (prompt-echoed)
// example of the same shape. Returns ok=false when no balanced span exists.
func lastBalancedSpan(s string, open, close byte) (start, end int, ok bool) {
	depth, spanStart := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			if depth == 0 {
				spanStart = i
			}
			depth++
		case close:
			if depth > 0 {
				depth--
				if depth == 0 && spanStart >= 0 {
					start, end, ok = spanStart, i, true // keep scanning for a later span
				}
			}
		}
	}
	return start, end, ok
}

// compile-time assertions that PhaseAdvisor satisfies both router ports.
var (
	_ router.Proposer = (*PhaseAdvisor)(nil)
	_ router.Planner  = (*PhaseAdvisor)(nil)
)
