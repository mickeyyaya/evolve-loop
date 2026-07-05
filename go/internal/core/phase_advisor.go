package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/panetrust"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
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
	bridge   Bridge
	identity AgentIdentity // ADR-0052 WS1-S1: the shared dispatch identity (cli/model/profile/persona/label)
	// writeArtifact persists the WS3-S1 redacted prompt/response capture.
	// Injectable so a write failure can be exercised (the capture is fail-open);
	// NewPhaseAdvisor defaults it to os.WriteFile.
	writeArtifact func(path string, data []byte) error
	// checkDepth is an injectable depth guard (defense-in-depth recursion check).
	// nil = skip the check. Wire AdvisorDepthExceeded via WithDepthCheck for production.
	checkDepth func(env map[string]string) bool
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
// When fn returns true for the dispatch env, advisorLaunch errors before bridge launch.
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

// NewPhaseAdvisor builds the routing brain over the given bridge. The cli/model
// FALLBACK is deep (opus) on the tmux Claude driver — composing the cycle and
// inventing phases is deep-reasoning work, not lightweight routing — but the
// composition root normally overrides both from the router profile + env so the
// brain is configurable to any CLI/model.
func NewPhaseAdvisor(bridge Bridge, opts ...PhaseAdvisorOption) *PhaseAdvisor {
	p := &PhaseAdvisor{
		bridge:        bridge,
		identity:      AgentIdentity{CLI: "claude-tmux", Model: "opus", AgentLabel: "router"},
		writeArtifact: writeArtifactAtomically,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Propose implements router.Proposer.
func (p *PhaseAdvisor) Propose(in router.RouteInput) (*router.Proposal, error) {
	resp, err := p.advisorLaunch(in, "routing proposer", "proposal", buildRoutingPrompt(in), "routing-proposal.json", "stdout", 0)
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
	return p.planWith(in, stageInitial)
}

// RePlan is the post-scout re-plan (ADR-0052 WS1-S3, the re-invokable seam): a
// SECOND whole-cycle plan computed once scout's handoff has populated in.Signals,
// so need is MEASURED rather than inferred from goal text (the upfront Plan runs
// with empty Signals). It shares planWith with the initial Plan — same
// compose→dispatch→parse path — writes a DISTINCT routing-replan.json artifact,
// and stamps replan_depth=1 on its decision span. The returned plan is ADVISORY;
// the kernel clamp re-validates it exactly as it does the initial plan.
//
// This slice provides ONLY the entrypoint — RePlan is not yet called by the loop.
// The WS2 hook point (post-scout, pre-build) and the EVOLVE_ROUTER_REPLAN dial
// drive it; until then it is byte-identical-off (never invoked).
func (p *PhaseAdvisor) RePlan(in router.RouteInput) (*router.PhasePlan, error) {
	return p.planWith(in, stagePostScout)
}

// planWith is the shared whole-cycle planning wiring for the initial Plan and the
// post-scout RePlan. The stage selects the raw artifact name (routing-plan.json
// vs routing-replan.json), the capture kind, and the re-plan depth on the span;
// everything else — prompt composition, dispatch, parse — is identical, so a
// regression in one stage is a regression in both.
//
// The advisor's raw plan artifact is distinct from the orchestrator's clamped
// phase-plan.json (written by recordPhasePlan): keeping them separate preserves
// both for forensics (advisory vs disposed).
func (p *PhaseAdvisor) planWith(in router.RouteInput, stage planStage) (*router.PhasePlan, error) {
	artifact := stage.artifactFile()
	resp, err := p.advisorLaunch(in, "phase advisor", stage.captureKind(), p.composePlanPrompt(in, artifact), artifact, "artifact", stage.replanDepth())
	if err != nil {
		return nil, err
	}
	plan, err := parsePhasePlan(resp.Stdout)
	if err != nil {
		return nil, fmt.Errorf("phase advisor: %w", err)
	}
	return plan, nil
}

// planStage selects which of the advisor's two whole-cycle planning calls is
// running (ADR-0052 WS1-S3). stageInitial is the cycle-start Plan (zero value =
// today's behavior); stagePostScout is the re-invokable RePlan. It is kept
// UNEXPORTED in core — ADR-0052 table 4.2 sketched it as a router.PlanStage, but
// the advisor's dispatch framing is core's concern and no other package consumes
// it, so exporting it would add cross-package coupling + an apicover surface with
// no caller. The router still owns the floor + canonical order it clamps against.
type planStage int

const (
	stageInitial planStage = iota
	stagePostScout
)

// artifactFile is the raw plan artifact name for the stage.
func (s planStage) artifactFile() string {
	if s == stagePostScout {
		return "routing-replan.json"
	}
	return "routing-plan.json"
}

// captureKind is the <kind> token embedded in the WS3 capture filenames
// (advisor-{prompt,response,span}-<kind>.*); isSafeArtifactKind confines it.
func (s planStage) captureKind() string {
	if s == stagePostScout {
		return "replan"
	}
	return "plan"
}

// replanDepth is the depth stamped on the decision span: a re-plan is one level
// deeper than the initial plan. WS2-S5 caps the live re-plan at this depth=1.
func (s planStage) replanDepth() int {
	if s == stagePostScout {
		return 1
	}
	return 0
}

// composePlanPrompt builds the whole-cycle planning prompt the uniform way: the
// persona body (agents/evolve-router.md — identity, job, mint guidance, output
// contract) followed by the DYNAMIC per-cycle context (objective digest, recall
// memory, catalog, decision rubric) appended in Go, exactly as a phase appends
// its cycle context. When no persona was injected it falls back to the legacy
// fully-inline framing (buildPlanPrompt) so the advisor still functions.
func (p *PhaseAdvisor) composePlanPrompt(in router.RouteInput, artifactFile string) string {
	if p.identity.Persona == "" {
		return buildPlanPrompt(in)
	}
	var b strings.Builder
	b.WriteString(p.identity.Persona)
	b.WriteString("\n\n---\n# This cycle\n\n")
	writeRoutingContext(&b, in)
	// WS2-S0b: inject the deterministic pre-plan recon when EVOLVE_ROUTER_RECON_DIGEST
	// is on. Off (default) ⇒ no gather, no render ⇒ byte-identical prompt. The gather
	// fails open, so even on, a degraded git only narrows the digest.
	if in.Cfg.ReconDigest {
		router.RenderReconDigest(&b, gatherPreplanRecon(in))
	}
	writeCatalog(&b, in.Catalog)
	writePlanResponseSchema(&b)
	// Instruct the ABSOLUTE artifact path — the same path advisorLaunch tells the
	// bridge to watch (filepath.Join(in.Workspace, artifactFile)). A relative path
	// lands in the REPL's cwd (under claude-tmux that is NOT the workspace — it
	// varies per cycle), so the bridge never sees it and the artifact-wait times out
	// → degrade to static (the cycle-210 failure). Absolute path = lands where watched.
	fmt.Fprintf(&b, "\nNow write your whole-cycle plan as a strict JSON array to %s (no prose, no fence).\n", filepath.Join(in.Workspace, artifactFile))
	return b.String()
}

// advisorLaunch is the shared wiring for Propose and Plan: it guards the
// required fields, resolves the router profile, and launches the bridge under
// the given completion contract. kind names the decision ("plan"/"proposal")
// and is embedded in the WS3 capture filenames (advisor-{prompt,response,span}-
// <kind>.*), so it MUST stay a confined token — isSafeArtifactKind enforces it.
//
// Plan uses completion="artifact" (the uniform, robust contract): the brain
// WRITES routing-plan.json and the bridge reads it back into resp.Stdout — same
// as every phase writes its report. Propose still uses completion="stdout"
// (ADR-0027 REPL-idle scrollback) pending its own unification. Either way, a
// failure returns an error and the caller degrades cleanly to the static path.
func (p *PhaseAdvisor) advisorLaunch(in router.RouteInput, errPfx, kind, prompt, artifactFile, completion string, replanDepth int) (BridgeResponse, error) {
	if p.bridge == nil {
		return BridgeResponse{}, fmt.Errorf("%s: nil bridge", errPfx)
	}
	if in.Workspace == "" {
		return BridgeResponse{}, fmt.Errorf("%s: empty workspace", errPfx)
	}
	// WS1-S2 recursion guard (defense-in-depth, ADR-0052 §4.3): refuse to dispatch
	// when the injected checkDepth guard signals a nested invocation, degrading the
	// cycle to the static path rather than nesting brains. The PRIMARY guard is the
	// mint denylist in mintConfigsFrom; this injectable seam is defense-in-depth.
	// Wire AdvisorDepthExceeded via WithDepthCheck for production behavior.
	if p.checkDepth != nil && p.checkDepth(in.Env) {
		return BridgeResponse{}, fmt.Errorf("%s: recursion guard: depth check failed", errPfx)
	}
	profile := p.identity.Profile
	if profile == "" && in.ProjectRoot != "" {
		profile = filepath.Join(in.ProjectRoot, ".evolve", "profiles", "router.json")
	}
	// Cycle-435: walk [identity.CLI]+profile.cli_fallback via llmroute.Dispatch —
	// the SAME chain-walk the runner uses for every ordinary phase — instead of a
	// single un-fallback-able Launch. identity.CLI (not llmroute.resolvePrimary,
	// which re-reads profile.cli) is the explicit primary so the composition
	// root's bench-aware CLI swap is honored (llmroute.ChainFor's H2 seam).
	plan := llmroute.ChainFor(p.identity.CLI, loadDispatchProfile(profile))
	var resp BridgeResponse
	dispatched := llmroute.Dispatch(plan, func(cli string) (int, error) {
		var launchErr error
		resp, launchErr = p.bridge.Launch(context.Background(), BridgeRequest{
			CLI:          cli,
			Profile:      profile,
			Model:        p.identity.Model,
			Prompt:       prompt,
			Workspace:    in.Workspace,
			Worktree:     in.ActiveWorktree,
			ArtifactPath: filepath.Join(in.Workspace, artifactFile),
			Completion:   completion,
			Agent:        p.identity.AgentLabel,
			Cycle:        in.Cycle,
			Env:          in.Env,
		})
		return resp.ExitCode, launchErr
	})
	if dispatched.Err != nil {
		return BridgeResponse{}, fmt.Errorf("%s: bridge launch: %w", errPfx, dispatched.Err)
	}
	// WS3-S1/S3: capture the redacted prompt+response and the decision span
	// BEFORE the caller parses, so the decision is debuggable + replayable.
	// Fail-open — never block the path.
	p.captureRedacted(in.Workspace, kind, prompt, resp.Stdout, resp.DurationMS, replanDepth)
	return resp, nil
}

// loadDispatchProfile parses the router profile at profilePath (the same path
// advisorLaunch passes as BridgeRequest.Profile) into a profiles.Profile so
// llmroute.ChainFor can read its cli_fallback chain. Fail-open (nil) on an
// empty path or any read/parse error — CLI-fallback resilience is advisory;
// a missing/malformed profile degrades to a single-candidate chain, never a
// dispatch failure.
func loadDispatchProfile(profilePath string) *profiles.Profile {
	if profilePath == "" {
		return nil
	}
	dir := filepath.Dir(profilePath)
	name := strings.TrimSuffix(filepath.Base(profilePath), ".json")
	prof, err := profiles.NewFromDir(dir).Get(name)
	if err != nil {
		return nil
	}
	return &prof
}

// AdvisorSpan is the OTel-GenAI decision span (ADR-0052 WS3-S3) persisted per
// advisor call as advisor-span-<kind>.json. Field keys follow the OTel GenAI
// semantic conventions so a collector can ingest the file directly. PromptSHA/
// ResponseSHA bind the REDACTED capture artifacts — the same identity the
// ledger (WS3-S2) and the replay path (WS3-S5) key off, so all three agree.
// ReplanDepth (WS1-S3) now varies: 0 for the initial Plan, 1 for the post-scout
// RePlan — so it records real behavior, not locked surface. It is always emitted
// (no omitempty): a depth of 0 is a meaningful "initial plan", not absence.
type AdvisorSpan struct {
	Model       string `json:"gen_ai.request.model"`
	System      string `json:"gen_ai.system"`
	PromptSHA   string `json:"prompt_sha"`
	ResponseSHA string `json:"response_sha"`
	DurationMS  int64  `json:"duration_ms"`
	ReplanDepth int    `json:"replan_depth"`
}

// captureRedacted persists the secret-redacted prompt and response plus the
// decision span for the given decision kind (WS3-S1 + WS3-S3). Best-effort /
// fail-open: each write's error is intentionally dropped — a forensic-capture
// failure must never become a routing outage (the advisor would otherwise
// degrade the whole cycle to the static path over a full disk). Only the
// PERSISTED COPY is redacted; the live prompt the advisor reasoned over is
// untouched. The response is redacted but not otherwise transformed, so WS3-S5
// can reparse it to the same plan.
func (p *PhaseAdvisor) captureRedacted(workspace, kind, prompt, response string, durationMS int64, replanDepth int) {
	if p.writeArtifact == nil || workspace == "" || !isSafeArtifactKind(kind) {
		return
	}
	rp := panetrust.RedactSecrets(prompt)
	rr := panetrust.RedactSecrets(response)
	_ = p.writeArtifact(filepath.Join(workspace, "advisor-prompt-"+kind+".txt"), []byte(rp))
	_ = p.writeArtifact(filepath.Join(workspace, "advisor-response-"+kind+".txt"), []byte(rr))
	if buf, err := json.Marshal(AdvisorSpan{
		Model:       p.identity.Model,
		System:      llmroute.Family(p.identity.CLI),
		PromptSHA:   sha256OfString(rp),
		ResponseSHA: sha256OfString(rr),
		DurationMS:  durationMS,
		ReplanDepth: replanDepth,
	}); err == nil {
		_ = p.writeArtifact(filepath.Join(workspace, "advisor-span-"+kind+".json"), buf)
	}
}

// sha256OfString returns the hex sha256 of s — the in-memory twin of
// bindArtifactSHA (which reads a file), so the span's bound SHA equals the
// ledger's bound SHA for the same redacted bytes.
func sha256OfString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// writeArtifactAtomically writes data via a temp file + rename, so a capture
// artifact on disk is always either absent or COMPLETE — never a truncated
// half-write (a crash mid-write leaves a stale .tmp, not a corrupt capture).
// This is what keeps the ledger's disk-read SHA (WS3-S2 bindArtifactSHA) equal
// to the span's in-memory SHA (WS3-S3 sha256OfString) for the same bytes, and
// matches the repo's atomic-write convention. Single-writer per (workspace,
// kind) per cycle, so the fixed .tmp suffix cannot collide.
func writeArtifactAtomically(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// isSafeArtifactKind constrains the <kind> token that names the capture files
// to a confined charset, so a future caller passing an externally-derived kind
// (e.g. "../../etc/x") can never escape the workspace via the file path. Today
// only the literals "plan"/"proposal" flow in; this guards the class, not the
// current trigger.
func isSafeArtifactKind(kind string) bool {
	if kind == "" || len(kind) > 32 {
		return false
	}
	for _, r := range kind {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
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

	if isFailureTransition(in) {
		writeFailureVocabulary(&b)
		b.WriteString("\n## Respond with STRICT JSON only (no prose, no markdown fence):\n")
		b.WriteString(`{"next_phase":"<phase>","insert_phases":["<phase>",...],"justification":"<one sentence>","learning_richness":"full|memo","recovery_action":"retry|end"}`)
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString("\n## Respond with STRICT JSON only (no prose, no markdown fence):\n")
	b.WriteString(`{"next_phase":"<phase>","insert_phases":["<phase>",...],"justification":"<one sentence>"}`)
	b.WriteString("\n")
	return b.String()
}

// isFailureTransition reports whether this routing call sits on a failure
// branch — the post-retro recovery decision or the audit-FAIL learning
// choice — where the failure vocabulary applies. Happy-path prompts stay
// byte-identical (prompt-prefix cache friendliness).
func isFailureTransition(in router.RouteInput) bool {
	switch strings.ToLower(strings.TrimSpace(in.Current)) {
	case "retrospective", "retro":
		return true
	case "audit":
		return in.Verdict == "FAIL"
	}
	return false
}

// writeFailureVocabulary renders the failure-path decision space (failure
// floor Phase 3). The floor is stated explicitly so the model does not
// waste tokens proposing what the kernel will clamp.
func writeFailureVocabulary(b *strings.Builder) {
	b.WriteString("\n## Failure-path vocabulary (this is a failure transition)\n")
	b.WriteString("- recovery_action: \"retry\" re-enters tdd to fix forward; \"end\" stops the cycle (e.g. budget nearly exhausted, systemic cause).\n")
	fmt.Fprintf(b, "- insert_phases may name %s to run BEFORE the retry (they precede tdd in canonical order).\n",
		strings.Join(router.FailureInsertPhases(), " or "))
	b.WriteString("- learning_richness: \"memo\" routes the lightweight memo phase instead of the full retrospective after an audit FAIL; \"full\" (default) keeps the retrospective. Learning ALWAYS happens — the deterministic floor records the failure regardless of your choice.\n")
	b.WriteString("- The failure-adapter's BLOCK verdicts are non-overridable: a blocked cycle ends no matter what you propose (the attempt is recorded as a clamp).\n")
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

	writePlanResponseSchema(&b)
	return b.String()
}

// writePlanResponseSchema renders the whole-cycle plan's response contract —
// the optional MINT block, the optional per-phase {cli,tier} dispatch
// proposal with the operator's model-tier policy, and the strict-JSON
// example — shared by composePlanPrompt (persona path, PRODUCTION) and
// buildPlanPrompt (legacy fallback), so the two prompt-assembly paths can
// never diverge again the way they did at #293 (the persona path never
// called this section at all).
func writePlanResponseSchema(b *strings.Builder) {
	b.WriteString("\n## Optionally MINT a new phase\n")
	b.WriteString("If an objective signal calls for work no existing phase covers, you MAY add an entry for a brand-new phase ")
	b.WriteString("by attaching a \"mint\" block. Give it a kebab-case phase name, an inline persona prompt, and a TIER ")
	b.WriteString("(fast|balanced|deep — never a raw model name). Minted phases are always optional and clamped by the kernel; ")
	b.WriteString("they can never reach ship without audit. Omit \"mint\" for existing phases. Minted phases default to ")
	b.WriteString("writes_source:true; set \"writes_source\":false only for phases that never edit source.\n")

	b.WriteString("\nYou MAY also propose a dispatch \"cli\" and abstract \"tier\" (fast|balanced|deep — never a raw model name) ")
	b.WriteString("for an EXISTING phase, honoring its allowed_clis/model_tier_envelope above when shown. Omit both to leave the phase's profile-pinned default unchanged.\n")

	b.WriteString("\n## Operator model-tier policy (apply when proposing a tier)\n")
	b.WriteString("- deep: judgment-heavy phases — build, tdd, audit, architecture/design, adversarial review.\n")
	b.WriteString("- balanced: review/scan/triage-class phases — the safe default for anything not covered below.\n")
	b.WriteString("- fast: ONLY mechanical phases (doc-sync, changelog-sync, locale-format-check, close-checklist) — NEVER for a phase that writes source, renders a verdict, or scopes work. When in doubt, propose HIGHER.\n")
	b.WriteString("- Judgment phases usually carry a min=balanced envelope above — don't fight the clamp with a low-tier proposal; propose a CLI only when deviating from the phase's profile default is justified.\n")

	b.WriteString("\n## Respond with STRICT JSON only (a bare array, no prose, no markdown fence):\n")
	b.WriteString(`[{"phase":"<phase>","run":true,"justification":"<one sentence>","cli":"<cli>","tier":"balanced"},`)
	b.WriteString(`{"phase":"<new-phase>","run":true,"justification":"<why>","mint":{"prompt":"<persona>","tier":"balanced","cli":"claude"}}]`)
	b.WriteString("\n")
}

// maxEnrichedCatalogCards bounds how many cards render with full metadata
// (categories + when-to-use hint) so a large plugin ecosystem cannot crowd the
// rubric out of the context window. Overflow phases stay SELECTable via a
// name-only line — a phase absent from the prompt cannot be selected at all.
const maxEnrichedCatalogCards = 12

// maxCardHintRunes caps a single card's when-to-use hint.
const maxCardHintRunes = 140

// writeCatalog renders the pre-defined phases the advisor may SELECT (WS3),
// biasing toward reuse over minting: a selectable phase already has a tuned
// persona + profile, so minting should be the exception (YAGNI for new phases).
// Cards carry the spec's advisor-facing metadata (ADR-0038); relevance judgment
// is the advisor LLM's job — Go only bounds the token cost. When the catalog
// exceeds the enriched cap, Optional (SELECTable) phases take the enriched
// slots — spine phases run via the mandatory config regardless. Deterministic
// order (catalog order, stable partition) ⇒ prompt-prefix-cache friendly.
// Emits nothing when the catalog is empty (legacy built-in-only path).
func writeCatalog(b *strings.Builder, cards []router.PhaseCard) {
	if len(cards) == 0 {
		return
	}
	b.WriteString("\n## Pre-defined phases you may SELECT (prefer these over minting)\n")
	b.WriteString("Each already exists with a tuned persona + profile. SELECT one by naming it in your plan ")
	b.WriteString("(no \"mint\" block). Only MINT a new phase when none of these fit the work.\n")

	enriched, overflow := cards, []router.PhaseCard(nil)
	if len(cards) > maxEnrichedCatalogCards {
		// Stable three-bucket priority for the enriched slots: a SELECTable card
		// with metadata has something to show; a metadata-less optional card
		// renders the same either way; spine cards run via the mandatory config
		// regardless of rendering.
		var withMeta, opt, rest []router.PhaseCard
		for _, c := range cards {
			switch {
			case c.Optional && (c.WhenToUse != "" || c.Description != "" || len(c.Categories) > 0):
				withMeta = append(withMeta, c)
			case c.Optional:
				opt = append(opt, c)
			default:
				rest = append(rest, c)
			}
		}
		ordered := append(withMeta, opt...)
		ordered = append(ordered, rest...)
		enriched, overflow = ordered[:maxEnrichedCatalogCards], ordered[maxEnrichedCatalogCards:]
	}
	for _, c := range enriched {
		writeCard(b, c)
	}
	if len(overflow) > 0 {
		b.WriteString("- All remaining selectable phases are in the \"## Phase Catalog — Core Values\" table (this prompt) and .evolve/phase-inventory.json (same SELECT rules; type/domain details there).\n")
	}
}

// writeCard renders one enriched catalog line:
//
//   - bug-reproduction [evaluate] (bugfix) — when: bugfix cycles, before tdd/build
//
// Metadata-less cards degrade to the legacy "- name [role]" form.
func writeCard(b *strings.Builder, c router.PhaseCard) {
	ws := ""
	if c.WritesSource {
		ws = ", writes-source"
	}
	fmt.Fprintf(b, "- %s [%s%s]", c.Name, c.Role, ws)
	if len(c.Categories) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(c.Categories, ", "))
	}
	hint := c.WhenToUse
	if hint == "" {
		hint = c.Description
	}
	if hint = truncateRunes(hint, maxCardHintRunes); hint != "" {
		fmt.Fprintf(b, " — when: %s", hint)
	}
	b.WriteString("\n")
	// Project this phase's own dispatch guardrails (cycle-436 MR1) so an
	// advisor proposing {cli,tier} for it has the legal bounds in hand instead
	// of guessing blind. Omitted entirely when the phase carries no per-phase
	// guardrail (the common case today).
	if len(c.AllowedCLIs) > 0 {
		fmt.Fprintf(b, "  allowed_clis: %s\n", strings.Join(c.AllowedCLIs, ", "))
	}
	if env := c.ModelTierEnvelope; env != nil {
		fmt.Fprintf(b, "  model_tier_envelope: {min: %s, default: %s, max: %s}\n", env.Min, env.Default, env.Max)
	}
}

// maxGoalTextChars bounds the goal text rendered into the advisor prompt so an
// oversized operator-pasted goal cannot crowd the catalog + rubric out of the
// context window.
const maxGoalTextChars = 4000

// truncateGoal trims surrounding whitespace and caps the goal at maxGoalTextChars
// (rune-safe), marking truncation. Empty/whitespace-only ⇒ "" (no Goal section).
func truncateGoal(s string) string { return truncateRunes(s, maxGoalTextChars) }

// truncateRunes trims surrounding whitespace and caps s at max runes, marking
// truncation. Shared by the goal section and the catalog card hints.
func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + " …[truncated]"
}

// writeRoutingContext writes the shared, deterministic decision context — cycle
// header, digested objective signals, available optional phases, and the
// decision rubric — consumed by both the per-transition prompt and the
// whole-cycle plan prompt. Deterministic string ⇒ prompt-prefix cache friendly.
func writeRoutingContext(b *strings.Builder, in router.RouteInput) {
	fmt.Fprintf(b, "## Cycle\n- cycle: %d\n- just_completed: %s\n- last_verdict: %s\n", in.Cycle, in.Current, in.Verdict)
	fmt.Fprintf(b, "- completed_phases: %s\n", strings.Join(in.Completed, ", "))
	fmt.Fprintf(b, "- mandatory_spine: %s\n", strings.Join(in.Cfg.Mandatory, ", "))
	fmt.Fprintf(b, "- max_optional_insertions: %d\n\n", in.Cfg.MaxInsertions)

	// The goal text (when threaded) is the brain's primary input for composing the
	// cycle: it lets the advisor judge whether the work is novel/cross-cutting
	// enough to warrant a design phase or a minted phase, instead of planning blind.
	// Capped so an oversized operator-pasted goal cannot push the catalog/rubric out
	// of the context window. Placed before the per-cycle signals: the goal is stable
	// across a run's cycles, so a stable section stays ahead of the volatile ones.
	if g := truncateGoal(in.GoalText); g != "" {
		fmt.Fprintf(b, "## Goal\n%s\n\n", g)
	}

	// Environmental CLI health: benched families mean dispatch chains start at
	// their fallback — the advisor should plan around the degraded family
	// (fewer inserts routed there; scope sized for the fallback carrying the
	// cycle) instead of discovering it one phase at a time (cycle-283).
	if len(in.BenchedCLIs) > 0 {
		b.WriteString("## CLI health (environmental)\n")
		benched := append([]router.BenchedCLI(nil), in.BenchedCLIs...)
		sort.Slice(benched, func(i, j int) bool { return benched[i].Family < benched[j].Family })
		for _, e := range benched {
			label := "benched"
			if strings.Contains(strings.ToLower(e.Reason), "exhaust") {
				label = "WALLED/unavailable"
			}
			fmt.Fprintf(b, "- %s: %s (%s) until %s — its dispatch chains start at the fallback CLI\n",
				e.Family, label, e.Reason, e.Until.UTC().Format("15:04Z"))
		}
		b.WriteString("\n")
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

	// Decision rubric — a PROJECTION of the structured routing data the
	// kernel walks (failure floor Phase 4b). Only the FORBIDDEN line stays
	// hardcoded — it is a kernel invariant, not phase data.
	b.WriteString("\n## Decision rubric (justify each optional phase by an objective signal)\n")
	writeRubricLines(b, in.Cfg)
	b.WriteString("FORBIDDEN: never propose reaching ship without audit. Any justification for skipping audit is rejected by the kernel.\n")
}

// writeRubricLines renders the decision rubric as a projection of the
// structured routing data the kernel already walks — conditional_mandatory
// rules (op negated: the rule says when the phase is PINNED; the rubric
// tells the advisor when it may be skipped), insert_when triggers, and the
// judgment-only routing.rubric_hint lines from the registry. One renderer,
// one home per belief: a threshold can never disagree between the walk and
// the prompt. Phases sort for a deterministic, prompt-prefix-cache-friendly
// string.
func writeRubricLines(b *strings.Builder, cfg config.RoutingConfig) {
	seen := make(map[string]struct{}, len(cfg.Triggers)+len(cfg.Conditional))
	for p := range cfg.Conditional {
		seen[p] = struct{}{}
	}
	for p := range cfg.Triggers {
		seen[p] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for p := range seen {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		if rule, ok := cfg.Conditional[p]; ok {
			if op, ok := negateOp(rule.Op); ok {
				fmt.Fprintf(b, "- %s %s %s → skip %s (conditional-mandatory exemption)\n", rule.Field, op, rule.Value, p)
			}
		}
		blk := cfg.Triggers[p]
		if len(blk.InsertWhen) > 0 {
			clauses := make([]string, len(blk.InsertWhen))
			for i, c := range blk.InsertWhen {
				clauses[i] = fmt.Sprintf("%s %s %v", c.Field, opSymbol(c.Op), c.Value)
			}
			fmt.Fprintf(b, "- %s → insert %s\n", strings.Join(clauses, " OR "), p)
		}
		for _, hint := range blk.RubricHint {
			fmt.Fprintf(b, "- %s\n", hint)
		}
	}
}

// opSymbol maps the condition-op vocabulary (router.evalCondition) to the
// comparison symbol the rubric prose uses. Already-symbolic and unknown ops
// render raw.
func opSymbol(op string) string {
	switch op {
	case "eq":
		return "=="
	case "ne":
		return "!="
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	}
	return op
}

// negateOp inverts a comparison and returns it symbolically. parseCondRule
// stores CondRule ops symbolically, but word-form ops (the Condition
// vocabulary) are accepted too so an in-process caller cannot silently lose
// an exemption line. Unknown ops render no line rather than a wrong one.
func negateOp(op string) (string, bool) {
	switch op {
	case "==", "eq":
		return "!=", true
	case "!=", "ne":
		return "==", true
	case ">", "gt":
		return "<=", true
	case ">=", "gte":
		return "<", true
	case "<", "lt":
		return ">=", true
	case "<=", "lte":
		return ">", true
	}
	return "", false
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

// maxCarryoverTodoActionRunesInPrompt bounds each rendered todo Action at the
// sole prompt-injection site (defense-in-depth). It guards the 54 oversized
// entries already on disk — which the creation-time caps in failure_learning.go
// cannot retroactively shrink — plus any future creation path this cycle does
// not touch.
const maxCarryoverTodoActionRunesInPrompt = 600

// carryoverPriorityRank maps a Priority string to a severity rank (higher =
// more severe). An unknown/malformed priority ranks lowest (0) so it sorts to
// the bottom without dropping the entry — the renderer stays total.
func carryoverPriorityRank(p string) int {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "P0":
		return 6
	case "P1", "H", "HIGH":
		return 5
	case "P2":
		return 4
	case "P3", "M", "MED", "MEDIUM":
		return 3
	case "L", "LOW":
		return 1
	default:
		return 0
	}
}

func writeCarryoverTodos(b *strings.Builder, todos []router.CarryoverTodo) {
	if len(todos) == 0 {
		return
	}
	b.WriteString("\n## Carryover todos from previous cycles (consider when selecting phases)\n")
	// When the array exceeds the count cap, render the HIGHEST-PRIORITY /
	// MOST-RECENT entries rather than a naive insertion-order (oldest-first)
	// prefix — the old todos[:20] silently hid the newest, most severe items
	// (e.g. cycle-505's leak) behind "N omitted". Sort a COPY (stable, so ties
	// keep on-disk order) — never mutate the caller's slice.
	ordered := append([]router.CarryoverTodo(nil), todos...)
	sort.SliceStable(ordered, func(i, j int) bool {
		ri, rj := carryoverPriorityRank(ordered[i].Priority), carryoverPriorityRank(ordered[j].Priority)
		if ri != rj {
			return ri > rj
		}
		return ordered[i].FirstSeenCycle > ordered[j].FirstSeenCycle
	})
	limit := min(len(ordered), maxCarryoverTodosInPrompt)
	for i := 0; i < limit; i++ {
		t := ordered[i]
		fmt.Fprintf(b, "- [%s] %s: %s (first_seen_cycle=%d, cycles_unpicked=%d)\n",
			t.Priority, t.ID, capRunes(t.Action, maxCarryoverTodoActionRunesInPrompt), t.FirstSeenCycle, t.CyclesUnpicked)
	}
	if len(ordered) > limit {
		fmt.Fprintf(b, "- ... %d more carryover todo(s) omitted from prompt\n", len(ordered)-limit)
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
	if prop.NextPhase == "" && len(prop.InsertPhases) == 0 &&
		prop.RecoveryAction == "" && prop.LearningRichness == "" {
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
	for i := range entries {
		entries[i].Tier = sanitizeAdvisorTier(entries[i].Tier)
	}
	return &router.PhasePlan{Entries: entries, MintPhases: mintConfigsFrom(entries)}, nil
}

// sanitizeAdvisorTier confines the advisor's OWN emitted tier to the strict
// canonical vocabulary — fast/balanced/deep/top, mirroring
// modelcatalog.CanonicalTiers ("top" = the frontier tier) — enforcing the
// driver_agnostic_model_routing invariant: the advisor
// proposes an ABSTRACT tier, never a raw or legacy model alias. Unlike
// policy.TierRank (which accepts "opus"/"sonnet"/"haiku" for an OPERATOR
// pin), an advisor-emitted alias or garbage value is dropped outright rather
// than translated — MR2's clamp downstream trusts this invariant instead of
// re-validating it. Empty stays empty (the common no-op case).
func sanitizeAdvisorTier(tier string) string {
	switch tier {
	case "fast", "balanced", "deep", "top":
		return tier
	default:
		return ""
	}
}

// reservedAdvisorNames are the control-plane identities a minted phase may never
// assume — the WS1-S2 recursion guard (PRIMARY). The advisor composes the
// executed spine; minting a router/advisor would let a brain schedule a brain,
// breaking the compose-vs-execute layering (ADR-0052 D1). The AgentLabel values
// the advisors dispatch under ("router"/"failure-advisor") are the canonical
// members; aliases cover the persona slug and bare role words.
var reservedAdvisorNames = map[string]struct{}{
	"router":                 {},
	"evolve-router":          {},
	"advisor":                {},
	"phase-advisor":          {},
	"failure-advisor":        {},
	"evolve-failure-advisor": {},
}

// reservedAdvisorMintReason returns a non-empty reason when name is a reserved
// control-plane identity (so a dropped mint is observable), or "" when the name
// is free to mint. Matched case-insensitively after trimming.
func reservedAdvisorMintReason(name string) string {
	if _, ok := reservedAdvisorNames[strings.ToLower(strings.TrimSpace(name))]; ok {
		return fmt.Sprintf("recursion guard: a minted phase may not assume the control-plane router/advisor identity %q", name)
	}
	return ""
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
//
// WS1-S2: a mint that would assume a reserved control-plane identity is dropped
// loudly (recursion guard) — the advisor proposes spine phases, never a router.
func mintConfigsFrom(entries []router.PhasePlanEntry) []phaseconfig.PhaseConfig {
	var out []phaseconfig.PhaseConfig
	for _, e := range entries {
		if e.Mint == nil {
			continue
		}
		if reason := reservedAdvisorMintReason(e.Phase); reason != "" {
			fmt.Fprintf(os.Stderr, "[advisor] dropping minted phase %q: %s\n", e.Phase, reason)
			continue
		}
		writesSource := true
		if e.Mint.WritesSource != nil {
			writesSource = *e.Mint.WritesSource
		}
		out = append(out, phaseconfig.PhaseConfig{
			PhaseSpec: phasespec.PhaseSpec{Name: e.Phase, WritesSource: writesSource},
			Dispatch:  phaseconfig.Dispatch{CLI: e.Mint.CLI, ModelTierDefault: e.Mint.Tier},
			Prompt:    e.Mint.Prompt,
		})
	}
	return out
}

// ReplayPlanFromResponse reparses a captured advisor response (WS3-S1's
// advisor-response-<kind>.txt) through the SAME parse + integrity-floor clamp
// the live planning path runs (parsePhasePlan → router.ClampPlanToFloorWith,
// the exact pair cyclerun.go uses), and returns the clamped plan + the clamps
// that fired. WS3-S5 replay uses it to prove a recorded response still
// reproduces the recorded phase-plan.json; WS4 builds its golden corpus on the
// same entry point, so a regression there is caught against the real floor —
// not a parallel reimplementation. An unparseable response is a loud error
// (detecting exactly that corruption is the point of replay).
func ReplayPlanFromResponse(raw string, in router.RouteInput, floor []string) (*router.PhasePlan, []router.Clamp, error) {
	plan, err := parsePhasePlan(raw)
	if err != nil {
		return nil, nil, err
	}
	clamped, clamps := router.ClampPlanToFloorWith(in, plan, floor, in.IntentRequired)
	return clamped, clamps, nil
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

// gatherPreplanRecon collects the deterministic pre-plan recon (ADR-0052
// WS2-S0b) for the whole-cycle plan. The I/O lives HERE (core, the I/O layer),
// reusing the gitexec seam — never re-rolling git invocation — and FAILS OPEN:
// a git error yields no changed files, so router.BuildReconDigest simply omits
// the file-derived facts. The backlog/carryover/goal facts come from the
// already-threaded RouteInput, so they survive even a git-less environment.
func gatherPreplanRecon(in router.RouteInput) router.ReconDigest {
	return router.BuildReconDigest(
		recentlyChangedFiles(in.ProjectRoot),
		in.GoalText,
		in.Signals.Scout.BacklogSize,
		len(in.CarryoverTodos),
	)
}

// recentlyChangedFiles returns the files touched across the last
// recentChangeCommits commits (duplicates kept — frequency = churn), or nil on
// any error / empty root (fail-open). One git call; reuses gitexec.
func recentlyChangedFiles(projectRoot string) []string {
	if projectRoot == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), reconGitTimeout)
	defer cancel()
	out, err := gitexec.Default(projectRoot).Output(ctx, "log", "-n", recentChangeCommits, "--name-only", "--pretty=format:")
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files
}
