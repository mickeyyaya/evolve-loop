// Package triage implements the cycle-scope task-selection phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// triage-specific variation points. See
// docs/architecture/packages/internal-phases-triage.md.
package triage

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
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

type hooks struct{ forbidden func(string) bool }

func (hooks) PhaseName() string                           { return string(core.PhaseTriage) }
func (hooks) AgentPromptName() string                     { return "evolve-triage" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "triage-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

// ShouldSkip delegates to the central PhasePolicy (config.Load reads
// EVOLVE_TRIAGE_DISABLE) rather than reading the env flag here.
func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseTriage)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseTDD), nil
}

func (h hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	carryover := req.Context["carryover_summary"]
	if req.Input.Active() {
		carryover = req.Input.CycleInputs().Carryover()
	}
	if carryover != "" {
		fmt.Fprintf(&b, "- carryover_summary: %s\n", carryover)
	}
	if scope := runner.LaneScope(req); scope != "" {
		fmt.Fprintf(&b, "- fleet_scope: this is one of several concurrent cycles; select ONLY tasks whose id is in this assigned set, ignore all others: %s\n", scope)
	}
	if ro := req.Context["recent_outcomes"]; ro != "" {
		fmt.Fprintf(&b, "- recent_outcomes: %s\n", ro)
	}
	if kind := req.Context[core.CtxKeyDeliverableKindDefault]; kind != "" {
		fmt.Fprintf(&b, "- deliverable_kind_default: %s\n", kind)
	}
	if root := req.Context[core.CtxKeyDeliverableRoot]; root != "" {
		fmt.Fprintf(&b, "- deliverable_root: %s\n", root)
	}
	if section := inboxBatchesSection(req.ProjectRoot, h.forbidden); section != "" {
		b.WriteString(section)
	}
	if section := premiseDriftSection(context.Background(), req.ProjectRoot, runner.LaneScope(req)); section != "" {
		b.WriteString(section)
	}
	ctx, cancel := context.WithTimeout(context.Background(), carryforwardCandidatesTimeout)
	defer cancel()
	if section := CarryforwardCandidatesSection(ctx, req.ProjectRoot, "main"); section != "" {
		b.WriteString(section)
	}
	return b.String()
}

const carryforwardCandidatesTimeout = 8 * time.Second

const carryforwardCandidatesMaxBranches = 40

// CarryforwardCandidatesSection renders the deterministic carry-forward
// candidate list for the triage prompt: local cycle-* branches under dir
// landable onto base, fail-open to "" on any error or when none qualify.
func CarryforwardCandidatesSection(ctx context.Context, dir, base string) string {
	if dir == "" || base == "" {
		return ""
	}
	cmd := exec.CommandContext(ctx, "git", "for-each-ref",
		"--sort=-committerdate", "--format=%(refname:short)", "refs/heads/cycle-*")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var refs []string
	for _, ref := range strings.Split(string(out), "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" || ref == base {
			continue
		}
		refs = append(refs, ref)
	}
	total := len(refs)
	truncated := total > carryforwardCandidatesMaxBranches
	if truncated {
		refs = refs[:carryforwardCandidatesMaxBranches]
	}
	var landable []string
	for _, ref := range refs {
		ok, err := core.CarryforwardCandidateLandable(ctx, dir, ref, base)
		if err != nil || !ok {
			continue
		}
		landable = append(landable, ref)
	}
	if len(landable) == 0 {
		return ""
	}
	var sect strings.Builder
	fmt.Fprintf(&sect, "- carryforward_candidates: %d local cycle-* branch(es) pre-screened as landable "+
		"(clean 3-way merge onto %s, not already superseded) by the deterministic filter — prefer cherry-picking "+
		"one of these over re-discovering the same work:\n", len(landable), base)
	for _, ref := range landable {
		fmt.Fprintf(&sect, "  - %s\n", ref)
	}
	if truncated {
		fmt.Fprintf(&sect, "  - (partial: %d of %d branches probed, newest-committed-first; %d older branch(es) not screened)\n",
			len(refs), total, total-len(refs))
	}
	return sect.String()
}

func inboxBatchesSection(projectRoot string, forbidden func(string) bool) string {
	if projectRoot == "" {
		return ""
	}
	inboxDir := filepath.Join(projectRoot, ".evolve", "inbox")
	items, _, err := inboxbatch.LoadDir(inboxDir)
	if err != nil || len(items) == 0 {
		return ""
	}
	if forbidden == nil {
		forbidden = guards.IsProtectedScope
	}
	rank := rankInputs(projectRoot)
	menu := inboxmover.RankLaneMenu(inboxmover.Options{InboxDir: inboxDir, Stderr: io.Discard}, items, forbidden, rank)
	var sect strings.Builder
	sect.WriteString(unknownClassNote(inboxrank.ClassWarnings(items, rank.Config)))
	sect.WriteString(selectableBatchesNote(menu.Ready, menu.Ranked))
	sect.WriteString(consoleRoutedNote(menu.Console))
	sect.WriteString(dependencyBlockedNote(menu.WaitingReasons))
	fmt.Fprintf(&sect, "- protected_surfaces: a top_n card must not name a path under a control-plane surface; drop such an item with reason "+
		"`protected-surface: <path>` (the host routes it to the console, and moves a card that still names one out of top_n): %s\n",
		strings.Join(protectedSurfaceFragments(), ", "))
	return sect.String()
}

func rankInputs(projectRoot string) inboxrank.Inputs {
	in, warnings := rankinputs.Load(paths.EvolveDirOf(projectRoot), time.Now())
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "[triage] WARN inbox rank: %s\n", w)
	}
	return in
}

func selectableBatchesNote(ready []inboxbatch.Item, ranked []inboxrank.Ranked) string {
	batches := inboxbatch.Classify(ready, inboxbatch.Config{Order: inboxrank.Sequence(ranked)})
	rendered := inboxbatch.RenderMarkdown(batches, inboxrank.Labels(ranked))
	if rendered == "" {
		return ""
	}
	return "- inbox_batches: the backlog below is pre-grouped by campaign/file-area; " +
		"prefer selecting a whole batch as top_n (its items share a worktree, build, and audit — " +
		"one cycle amortizes the pipeline across them) over cherry-picking single items across batches. " +
		"Batches come in inbox-rank order, the computed priority `evolve inbox rank` shows, and each item's line gives its score and top factor:\n" +
		phaseidentity.WrapPasted(rendered)
}

func consoleRoutedNote(console []inboxbatch.Item) string {
	if len(console) == 0 {
		return ""
	}
	ids := make([]string, len(console))
	for i, it := range console {
		ids[i] = it.ID
	}
	return fmt.Sprintf("- console_routed_excluded: %d operator-owned item(s) NOT selectable (route, pipeline-* kind, or protected fix surface; the claim floor refuses them): %s\n",
		len(console), strings.Join(ids, ", "))
}

func unknownClassNote(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	return fmt.Sprintf("- unknown_priority_class: %d queued item(s) carry no class the policy's class_order names, so the rank puts them below every class: %s\n",
		len(warnings), strings.Join(warnings, "; "))
}

func dependencyBlockedNote(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	return fmt.Sprintf("- dependency_blocked: %d item(s) NOT selectable until their declared dependency lands: %s\n",
		len(reasons), strings.Join(reasons, "; "))
}

func protectedSurfaceFragments() []string {
	out := make([]string, 0, len(guards.ProtectedSurfaceManifest))
	for _, e := range guards.ProtectedSurfaceManifest {
		out = append(out, e.Fragment)
	}
	return out
}

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, diags := specrunner.EvaluateClassify(artifact, &phasespec.ClassifyRules{
		RequireSections: []string{phasecontract.Triage.Sections[0].Canonical},
		FailIfEmpty:     true,
	})
	if verdict != core.VerdictPASS {
		return verdict, diags, string(core.PhaseTDD)
	}
	trimmed := strings.TrimSpace(artifact)
	body, hasSection := topNSectionBody(trimmed)
	if hasSection && !listItemRE.MatchString(body) {
		signals, err := router.Digest(req.Workspace, []string{string(core.PhaseTriage)})
		if err == nil && signals.HasEmptyTriageCommitment() {
			return core.VerdictPASS, nil, string(core.PhaseTDD)
		}
	}
	if !hasSection || !listItemRE.MatchString(body) {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "## top_n section has no list items",
			Code:     cyclestate.DiagCodeTriageTopNEmpty,
		}}, string(core.PhaseTDD)
	}
	if cards := protectedTopNCards(body, h.forbidden); len(cards) > 0 {
		if err := routeProtectedCards(filepath.Join(req.Workspace, "triage-decision.json"), cards, boundItems(req.Workspace)); err != nil {
			return core.VerdictFAIL, []core.Diagnostic{{
				Severity: cyclestate.SeverityError,
				Message: fmt.Sprintf("top_n card %q names protected surface %q and its console route could not be recorded: %v",
					cards[0].ID, cards[0].Path, err),
				Code:    cyclestate.DiagCodeTriageProtectedSurface,
				Subject: cards[0].ID,
			}}, string(core.PhaseTDD)
		}
		diags = append(diags, routedCardDiagnostics(cards)...)
	}
	unifiedDiags, err := processUnifiedCommitment(req)
	if err != nil {
		return core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message:  "unified_commitment processing failed: " + err.Error(),
			Code:     cyclestate.DiagCodeTriageCommitmentInvalid,
		}}, string(core.PhaseTDD)
	}
	return core.VerdictPASS, append(diags, unifiedDiags...), string(core.PhaseTDD)
}

// topNSectionBody returns the ## top_n section body, or ok=false when the
// heading is absent. trimmed must already be whitespace-trimmed.
func topNSectionBody(trimmed string) (body string, ok bool) {
	loc := topNHeadingRE.FindStringIndex(trimmed)
	if loc == nil {
		return "", false
	}
	body = trimmed[loc[1]:]
	if next := nextHeadingRE.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}
	return body, true
}

// topNItemIDRE captures the id token of a "- id: description ..." top_n list
// item — the shape triage-report.md's real output uses.
var topNItemIDRE = regexp.MustCompile(`(?m)^[-*]\s+([^:\n]+):`)

var filesFieldRE = regexp.MustCompile(`files=(?:\{([^}]*)\}|([^,\n]*))`)

// Config holds the dependencies for constructing a triage Phase.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	// ContractVerifier is the deliverables gate's verifier accessor for the
	// verdict engine (runner.Options.ContractVerifier); nil uses the
	// catalog-aware default.
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung; the
	// zero value (StageOff) is byte-identical.
	// See ADR-0050.
	PhaseIO config.Stage
	// CompactPrompts strips the on-demand reference tail from the disk-loaded
	// agent doc before dispatch; it flows from workflow.compact_prompts
	// (policy.json), never a literal here.
	CompactPrompts bool
	// LaneForbidden marks the declared paths no lane can change; nil judges protected surface only.
	LaneForbidden func(string) bool
}

func boundItems(workspace string) []string {
	ids, _ := committedset.Committed(workspace)
	return ids
}

func hooksFor(c Config) hooks { return hooks{forbidden: c.LaneForbidden} }

// Phase is the triage cycle-scope task-selection phase, a runner.BaseRunner
// specialized with the triage-specific hooks.
type Phase struct{ *runner.BaseRunner }

// New constructs a triage Phase from c, wiring the triage hooks, bridge,
// prompts, clock, and PhaseIO stage into a runner.BaseRunner.
func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:            hooksFor(c),
			Bridge:           c.Bridge,
			ContractVerifier: c.ContractVerifier,
			HostEffects:      c.HostEffects,
			Prompts:          c.Prompts,
			NowFn:            c.NowFn,
			PhaseIO:          c.PhaseIO,
			CompactPrompts:   c.CompactPrompts,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseTriage), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot, nil),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
