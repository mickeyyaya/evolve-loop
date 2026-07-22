// Package triage implements the cycle-scope task-selection phase. The
// phase boilerplate lives in internal/phases/runner; this file only
// encodes triage-specific variation points.
//
// Skip semantics (Skipper interface):
//   - EVOLVE_TRIAGE_DISABLE=1 → SKIPPED, NextPhase=tdd, no bridge call
//
// Verdict mapping:
//   - empty artifact → FAIL
//   - missing "## top_n" heading → FAIL
//   - "## top_n" section with no list items → FAIL
//   - "## top_n" with ≥1 list item → PASS
package triage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// topNHeadingRE locates the selection-section heading (phasecontract.Triage,
// single source).
var topNHeadingRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(phasecontract.Triage.Sections[0].Canonical) + `\b`)

// listItemRE matches a single non-empty Markdown list item line.
var listItemRE = regexp.MustCompile(`(?m)^[-*]\s+\S`)

// nextHeadingRE finds the next "## " section heading.
var nextHeadingRE = regexp.MustCompile(`(?m)^## `)

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseTriage) }
func (hooks) AgentPromptName() string                     { return "evolve-triage" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "triage-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

// ShouldSkip delegates the enable/skip decision to the central PhasePolicy
// (config.Load is the sole reader of EVOLVE_TRIAGE_DISABLE), instead of
// reading the env flag literal here. Legacy posture preserved: triage runs
// unless disabled.
func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseTriage)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseTDD), nil
}

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	// ADR-0050 §3.10 Slice 2: typed envelope at enforce, legacy Context below it
	// (byte-identical — Active() is false unless enforce).
	carryover := req.Context["carryover_summary"]
	if req.Input.Active() {
		carryover = req.Input.CycleInputs().Carryover()
	}
	if carryover != "" {
		fmt.Fprintf(&b, "- carryover_summary: %s\n", carryover)
	}
	// ADR-0049 E: under `evolve fleet --plan` this cycle is one of several running
	// concurrently, each assigned a DISJOINT set of tasks. Steer selection to ONLY
	// the assigned ids so two cycles never pick work touching the same files.
	// Resolution + control-char sanitization live in runner.LaneScope (shared
	// with scout/build/tdd since cycle-776).
	if scope := runner.LaneScope(req); scope != "" {
		fmt.Fprintf(&b, "- fleet_scope: this is one of several concurrent cycles; select ONLY tasks whose id is in this assigned set, ignore all others: %s\n", scope)
	}
	// Chronicle S3 (digest stage=enforce): the orchestrator seeds
	// Context["recent_outcomes"] with the recent-outcomes digest at cycle
	// start. Appended AFTER the stable prefix lines (cache-friendly ordering);
	// absent/empty key keeps the prompt byte-identical (shadow/off pin).
	if ro := req.Context["recent_outcomes"]; ro != "" {
		fmt.Fprintf(&b, "- recent_outcomes: %s\n", ro)
	}
	// Inbox batch classifier (2026-07-16): one-item-per-cycle consumption pays
	// the full pipeline per item, so internal/inboxbatch DETERMINISTICALLY
	// groups the backlog by campaign / package area / explicit links (Core
	// Rule 5 — grouping is mechanical Go; only the PICK stays LLM judgment).
	// Rendered last (the section varies as items land — keeps the stable
	// prefix cache-friendly); an empty/missing/unreadable inbox keeps the
	// prompt byte-identical (fail-open: a broken backlog must not block
	// triage, which still reads the inbox directly).
	if section := inboxBatchesSection(req.ProjectRoot); section != "" {
		b.WriteString(section)
	}
	return b.String()
}

// inboxBatchesSection renders the deterministic backlog grouping for the
// triage prompt, or "" when there is nothing to group (the byte-identity pin).
// An empty projectRoot returns "" rather than resolving a CWD-relative path —
// the dual-root landmine PhaseRequest's own docs warn about.
func inboxBatchesSection(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	items, _, err := inboxbatch.LoadDir(filepath.Join(projectRoot, ".evolve", "inbox"))
	if err != nil || len(items) == 0 {
		return ""
	}
	// ADR-0074 I1: console-routed items (route:"console-*" or a protected fix
	// surface) are operator-owned — lanes must never see them as selectable.
	// The exclusion is loud (ids listed) so triage knows the work exists;
	// inboxmover.Claim is the enforcement backstop if a pick slips through.
	dispatchable, console, _ := inboxbatch.PartitionConsole(items, guards.IsProtectedSurface)
	var sect strings.Builder
	if rendered := inboxbatch.RenderMarkdown(inboxbatch.Classify(dispatchable, inboxbatch.Config{})); rendered != "" {
		sect.WriteString("- inbox_batches: the backlog below is pre-grouped by campaign/file-area/links; " +
			"prefer selecting a whole batch as top_n (its items share a worktree, build, and audit — " +
			"one cycle amortizes the pipeline across them) over cherry-picking single items across batches:\n" +
			rendered)
	}
	if len(console) > 0 {
		ids := make([]string, len(console))
		for i, it := range console {
			ids[i] = it.ID
		}
		fmt.Fprintf(&sect, "- console_routed_excluded: %d operator-owned item(s) NOT selectable (route/protected fix surface; the claim floor refuses them): %s\n",
			len(console), strings.Join(ids, ", "))
	}
	return sect.String()
}

func (hooks) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	// EvaluateClassify handles the empty-artifact and section-presence checks.
	verdict, diags := specrunner.EvaluateClassify(artifact, &phasespec.ClassifyRules{
		RequireSections: []string{phasecontract.Triage.Sections[0].Canonical},
		FailIfEmpty:     true,
	})
	if verdict != core.VerdictPASS {
		return verdict, diags, string(core.PhaseTDD)
	}
	// Extra triage invariant: ## top_n must contain at least one list item.
	if !hasTopNItems(strings.TrimSpace(artifact)) {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "## top_n section has no list items",
		}}, string(core.PhaseTDD)
	}
	return core.VerdictPASS, nil, string(core.PhaseTDD)
}

func hasTopNItems(trimmed string) bool {
	loc := topNHeadingRE.FindStringIndex(trimmed)
	if loc == nil {
		return false
	}
	body := trimmed[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return listItemRE.MatchString(body)
}

// Config holds the dependencies for constructing a triage Phase: the bridge
// used to dispatch the agent, the prompt loader, an optional clock, and the
// PhaseIO stage.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
	// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
	PhaseIO config.Stage
	// CompactPrompts strips the on-demand reference tail from the disk-loaded agent
	// doc before dispatch. Value flows from workflow.compact_prompts (policy.json);
	// never set to a bare literal here (standing rule: phase-settings-from-config).
	CompactPrompts bool
}

// Phase is the triage cycle-scope task-selection phase, a runner.BaseRunner
// specialized with the triage-specific hooks.
type Phase struct{ *runner.BaseRunner }

// New constructs a triage Phase from c, wiring the triage hooks, bridge,
// prompts, clock, and PhaseIO stage into a runner.BaseRunner.
func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:          hooks{},
			Bridge:         c.Bridge,
			Prompts:        c.Prompts,
			NowFn:          c.NowFn,
			PhaseIO:        c.PhaseIO,
			CompactPrompts: c.CompactPrompts,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseTriage), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
