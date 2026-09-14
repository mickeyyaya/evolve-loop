package advisor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

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

	WriteRoutingContext(&b, in)

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
// It is the legacy inline framing ComposePlanPrompt falls back to without a
// persona.
func buildPlanPrompt(in router.RouteInput) string {
	var b strings.Builder
	b.WriteString("You are the evolve-loop PHASE ADVISOR. The model proposes; the kernel disposes.\n")
	b.WriteString("From the objective signals below, decide which phases should RUN this cycle and which to SKIP, ")
	b.WriteString("with a one-sentence justification per phase. Your plan is ADVISORY and will be clamped to the ")
	b.WriteString("integrity floor (ship requires a real PASS audit bound to the built tree) — a plan that reaches ")
	b.WriteString("ship without audit is rejected by the kernel.\n\n")

	WriteRoutingContext(&b, in)

	WriteCatalogWithOnDemand(&b, in.Catalog, in.OnDemandPhases)

	writePlanResponseSchema(&b)
	return b.String()
}

// writePlanResponseSchema renders the whole-cycle plan's response contract —
// the optional MINT block, the optional per-phase {cli,tier} dispatch
// proposal with the operator's model-tier policy, and the strict-JSON
// example — shared by ComposePlanPrompt (persona path, PRODUCTION) and
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
	b.WriteString("ALSO supply \"description\" (one line: what the phase produces) and \"when_to_use\" (the signal that should trigger ")
	b.WriteString("SELECTing it) — this is the SELECT metadata a later cycle reads to reuse your phase instead of minting a duplicate.\n")

	b.WriteString("\nYou MAY also propose a dispatch \"cli\" and abstract \"tier\" (fast|balanced|deep — never a raw model name) ")
	b.WriteString("for an EXISTING phase, honoring its allowed_clis/model_tier_envelope above when shown. Omit both to leave the phase's profile-pinned default unchanged.\n")

	b.WriteString("\n## Operator model-tier policy (apply when proposing a tier)\n")
	b.WriteString("- deep: judgment-heavy phases — build, tdd, audit, architecture/design, adversarial review.\n")
	b.WriteString("- balanced: review/scan/triage-class phases — the safe default for anything not covered below.\n")
	b.WriteString("- fast: ONLY mechanical phases (doc-sync, changelog-sync, locale-format-check, close-checklist) — NEVER for a phase that writes source, renders a verdict, or scopes work. When in doubt, propose HIGHER.\n")
	b.WriteString("- Judgment phases usually carry a min=balanced envelope above — don't fight the clamp with a low-tier proposal; propose a CLI only when deviating from the phase's profile default is justified.\n")

	b.WriteString("\n## Respond with STRICT JSON only (a bare array, no prose, no markdown fence):\n")
	b.WriteString(`[{"phase":"<phase>","run":true,"justification":"<one sentence>","cli":"<cli>","tier":"balanced"},`)
	b.WriteString(`{"phase":"<new-phase>","run":true,"justification":"<why>","mint":{"prompt":"<persona>","tier":"balanced","cli":"claude","description":"<what it produces>","when_to_use":"<when to select it>"}}]`)
	b.WriteString("\n")
}

// ComposePlanPrompt builds the whole-cycle planning prompt the uniform way: the
// persona body (agents/evolve-router.md — identity, job, mint guidance, output
// contract) followed by the DYNAMIC per-cycle context (objective digest, recall
// memory, catalog, decision rubric) appended in Go, exactly as a phase appends
// its cycle context. When no persona was injected it falls back to the legacy
// fully-inline framing (buildPlanPrompt) so the advisor still functions.
// artifactFile is the raw plan artifact the prompt instructs the model to
// write — the ABSOLUTE workspace path the launch also tells the bridge to
// watch (a relative path lands in the REPL's cwd, which under claude-tmux is
// NOT the workspace, so the bridge never sees it — the cycle-210 failure).
// It is the exported entry the seam's test facade reaches with an arbitrary
// artifact name, so it does the ONE string→decision map for the compose-time
// stamp; production (Plan/RePlan) composes from its decision directly.
func (a *Advisor) ComposePlanPrompt(in router.RouteInput, artifactFile string) string {
	return a.composePlanPrompt(in, decisionForArtifact(artifactFile), artifactFile)
}

// composePlanPrompt is the composer both entries share: the decision stamps
// any compose-time event, the artifact name lands in the closing instruction.
func (a *Advisor) composePlanPrompt(in router.RouteInput, d decision, artifactFile string) string {
	if a.identity.Persona == "" {
		return buildPlanPrompt(in)
	}
	var b strings.Builder
	b.WriteString(a.identity.Persona)
	b.WriteString("\n\n---\n# This cycle\n\n")
	WriteRoutingContext(&b, in)
	// WS2-S0b: inject the deterministic pre-plan recon when cfg.ReconDigest is
	// on. Off (default) ⇒ no gather, no render ⇒ byte-identical prompt. The
	// gather fails open, so even on, a degraded git only narrows the digest.
	if in.Cfg.ReconDigest {
		router.RenderReconDigest(&b, a.gatherRecon(in, d))
	}
	WriteCatalogWithOnDemand(&b, in.Catalog, in.OnDemandPhases)
	writePlanResponseSchema(&b)
	fmt.Fprintf(&b, "\nNow write your whole-cycle plan as a strict JSON array to %s (no prose, no fence).\n", filepath.Join(in.Workspace, artifactFile))
	return b.String()
}

// gatherRecon collects the deterministic pre-plan recon (ADR-0052 WS2-S0b)
// through the injected recent-files reader (core's git reader in production;
// the Null Object in a git-less environment) and FAILS OPEN: a reader error
// is one ADVISOR_RECON_GIT_FAILED and yields no changed files, so
// router.BuildReconDigest simply omits the file-derived facts. The backlog/
// carryover/goal facts come from the already-threaded RouteInput, so they
// survive regardless. An empty project root reads nothing (configuration,
// not a fault).
func (a *Advisor) gatherRecon(in router.RouteInput, d decision) router.ReconDigest {
	var files []string
	if in.ProjectRoot != "" {
		var err error
		if files, err = a.recentFiles(in.ProjectRoot); err != nil {
			a.warn(in, d, CodeReconGitFailed, err.Error(), map[string]string{"step": stepCompose, "project_root": in.ProjectRoot})
			files = nil
		}
	}
	return router.BuildReconDigest(files, in.GoalText, in.Signals.Scout.BacklogSize, len(in.CarryoverTodos))
}
