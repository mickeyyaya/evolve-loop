// Package audit implements the EGPS gate phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// audit-specific variation points.
//
// Audit is the EGPS gate: PASS requires BOTH a parseable PASS verdict
// in audit-report.md AND red_count == 0 in acs-verdict.json.
// policy.json workflow.strict_audit additionally promotes WARN to FAIL.
//
// Verdict mapping:
//   - empty artifact / no parseable verdict declaration → FAIL
//   - acs-verdict.json missing or unparseable → FAIL + error diag
//   - acs-verdict.json red_count > 0 → FAIL + EGPS diag
//   - WARN + workflow.strict_audit → FAIL
//   - otherwise → whatever verdict the audit-report.md declares (PASS/WARN/FAIL/SKIPPED)
//
// The verdict declaration is recognized in several agent-produced shapes —
// canonical "## Verdict\n**PASS**" AND single-line variants like
// "**Verdict: PASS**" or "Verdict: PASS". Prose formatting varies by CLI, so
// the gate must not hinge on one exact shape: a genuine PASS written as
// "**Verdict: PASS**" with red_count==0 must not be mis-graded FAIL (the
// cycle-148 silent-no-ship bug). When the verdict is unparseable but the EGPS
// suite is green, a loud diagnostic is emitted (never a silent FAIL).
//
// Default model is "opus" for adversarial cross-family diversity from
// the build phase's Sonnet.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/regressiontia"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/internal/skillcheck"
)

// auditReportMaxBytes bounds the SIZE of audit-report.md, the same way
// defect_ledger.go's defectLedgerMaxEntries/defectTextMaxRunes bound the
// ledger: bound it, RECORD the overflow, never silently drop. The report is
// re-read in full at ship time and SHA-bound there
// (internal/phases/ship/audit.go), and the next cycle's handoff carries the
// prior audit forward, so an unbounded ## Issues table compounds token cost on
// every downstream read.
//
// Two properties are load-bearing. The overflow diagnostic is severity
// "warning", NEVER "error": core's errorSeverityMessages keys off
// Severity=="error" to build AuditFailReasons, so an error here would convert a
// merely verbose report into a dossier-visible failure. And the check never
// touches the file on disk — ship SHA-binds those exact bytes, so a truncating
// cap would break the integrity check it was meant to protect.
//
// 32KiB is ~1.5x the largest audit-report.md observed across 256 recorded runs
// (max 22,035 bytes, p90 15,777): high enough that a normal report never trips
// it, low enough to catch a runaway table.
const auditReportMaxBytes = 32 * 1024

// The regex-on-prose verdict fallback (canonical "## Verdict\n**PASS**" and
// the colon-bearing inline forms) is single-homed in reportdoc.Verdict since
// ADR-0095: the dashboard's round history and the repair-brief seed read the
// SAME grammar, so what the gate scores and what the operator/rebuilder sees
// cannot disagree. reportdoc scans visible lines only (fenced, indented and
// HTML-commented content stripped), so an embedded template can no longer
// declare a verdict here either.

// hooks carries the audit phase's variation points. genVerdict is the
// seam that generates acs-verdict.json when it is absent (cycle-138/139
// fix): the autonomous loop never ran `evolve acs suite`, so the EGPS
// gate was forced to FAIL on the missing file every cycle. nil = no
// generation (a pre-staged file is then required, the legacy behavior).
type hooks struct {
	genVerdict func(req core.PhaseRequest) error
	// explanationCheck independently verifies the Build-explanation handoff
	// after the auditor has finished. A failure overrides the native Audit
	// response without masquerading as an EGPS predicate. nil keeps legacy unit
	// fixtures unchanged.
	explanationCheck func(req core.PhaseRequest) error
	// gofmtCheck reports the worktree's .go files that are not gofmt -s clean.
	// It is the CI-parity gate that stops a cycle shipping a gofmt regression
	// to main (cycles 339-341 shipped CI-red because the cycle-scoped audit
	// never ran gofmt over the generated go/acs/cycle<N>/*.go files). nil = no
	// gofmt gate (legacy/tests). The registry default wires gofmtCheckDefault.
	gofmtCheck func(req core.PhaseRequest) ([]string, error)
	// skillsDriftCheck reports the worktree's SKILL.md files whose generated
	// phase-facts region has drifted from its SSOTs (profiles/registry/
	// phasecontract). A cycle that edits .evolve/profiles/*.json without
	// regenerating would FAIL the CI TestSkills_NoDrift gate (cycle 339), so the
	// drift must FAIL audit. nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault (in-process skillcheck.Check — no subprocess).
	skillsDriftCheck func(req core.PhaseRequest) ([]string, error)
	// goVetCheck / acsDurableCheck / apicoverEnforceCheck are the CI-parity
	// gates: each runs the EXACT whole-repo CI command (go vet ./..., -tags acs
	// acs-durable, apicover -enforce over touched-enforced packages) against the
	// cycle worktree and FAILs audit on offenders — closing the "per-cycle proof
	// ≠ repo-wide CI gate" gap (import cycles, flagregistry/flag-ceiling, unnamed
	// exports). nil = no gate (tests). NewDefaultWithStageCompact wires the
	// *Default impls (subprocess); each fails OPEN (warning) if it cannot run.
	goVetCheck           func(req core.PhaseRequest) ([]string, error)
	acsDurableCheck      func(req core.PhaseRequest) ([]string, error)
	apicoverEnforceCheck func(req core.PhaseRequest) ([]string, error)
	// integrationTierCheck runs the `-tags integration` test tier (go.yml's
	// "test … incl. integration tier" step) against the cycle worktree. It
	// closes the parity hole one tier up from goVetCheck: the fleet soak
	// (TestFleetSoak) went red in CI while the per-cycle audit stayed green
	// because ciparity ran vet/acs-durable/apicover but never the integration
	// tier. nil = no gate. NewDefaultWithStageCompact wires the *Default impl.
	integrationTierCheck func(req core.PhaseRequest) ([]string, error)
	// apicoverNewPkgGraduationCheck reports changed go/internal/<pkg> packages
	// that are new this cycle and absent from .apicover-enforce — the blind spot
	// IntersectEnforced silently drops (a new package cannot yet be in the
	// enforce list, so the touched∩enforced scoping never inspects it). Any such
	// ungraduated package FAILs audit fail-fast, closing the recurring
	// warnship_apicover_ci_gap. nil = no gate. NewDefaultWithStageCompact wires
	// apicoverNewPackageGraduationDefault.
	apicoverNewPkgGraduationCheck func(req core.PhaseRequest) ([]string, error)
	// phaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). At >= StageEnforce the evolve-verdict sentinel is mandatory —
	// the legacy prose/regex fallbacks are gated off. Zero value (StageOff) keeps
	// every path active, byte-identical.
	phaseIO config.Stage
}

func (hooks) PhaseName() string       { return string(core.PhaseAudit) }
func (hooks) AgentPromptName() string { return "evolve-auditor" }

// SecondaryArtifacts (runner.SecondaryArtifactsProvider): on a continuation
// cycle — the workspace carries the adopter-written continuation manifest —
// the audit contract also requires defect-dispositions.json (ADR-0074 /
// cycle-1285 lineage accounting). Declaring it holds session teardown until
// the auditor writes it, closing the write-one-artifact-and-die class that
// failed cycles 1397-1429. Non-continuation audits return nil: byte-identical
// legacy behavior.
func (hooks) SecondaryArtifacts(req core.PhaseRequest) []string {
	if _, err := os.Stat(filepath.Join(req.Workspace, "continuation-manifest.json")); err != nil {
		return nil
	}
	return []string{filepath.Join(req.Workspace, "defect-dispositions.json")}
}

func (hooks) ArtifactFilename(_ core.PhaseRequest) string {
	return phasecontract.ArtifactFilename(string(core.PhaseAudit))
}
func (hooks) DefaultModel() string { return "opus" } // Adversarial cross-family from Builder's Sonnet.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	// Continuations are TOLD their inherited OPEN defect ids (2026-08-10
	// investigation: auditors were graded against ids they were never shown).
	b.WriteString(inheritedDefectsPromptBlock(req))
	b.WriteString(chainExamplePromptBlock())
	return b.String()
}

// chainExamplePromptBlock SHOWS the auditor the reasoning-chain shape
// (ADR-0088) instead of only describing it.
//
// Measured, not assumed: the first shadow wave dispatched three audits with
// byte-identical prompts that all carried the persona's chain instruction —
// delivery worked, compaction stripped nothing — and one of the three emitted a
// chain. The persona describes the format in prose and cannot show it, because
// the combined line budget has five lines of headroom and the example is nine.
//
// Injecting it here costs no budget and closes the drift hole a review raised
// as a BLOCK: auditchain.ChainBlockExample is now the persona's illustration,
// the parser's own round-trip fixture, and the dispatched text — one constant,
// three legs (ADR-0084 I2). The parser is tail-anchored, so an auditor that
// echoes this block above its real one has the echo ignored.
func chainExamplePromptBlock() string {
	return "\n## Reasoning chain — emit exactly this shape (ADR-0088)\n\n" +
		"Your verdict is the CONCLUSION of this chain. Replace every status and finding with your own;\n" +
		"cite something a third party can open. A link you could not check is `unverifiable`, never\n" +
		"`coherent`. A link you omit is treated as decisive against the cycle.\n\n" +
		auditchain.ChainBlockExample + "\n"
}

func (h hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, verdictFound := extractAuditVerdict(artifact, h.phaseIO)
	// narrative is the auditor's OWN verdict, captured BEFORE any gate override
	// below can overwrite it. The override is correct — a deterministic gate must
	// outrank prose (cycles 339-341) — but silently discarding the disagreement
	// leaves an operator unable to tell a genuine defect from a POISONED
	// predicate the auditor itself read as clean (cycles 1107/1116/1117).
	narrative := verdict
	if !verdictFound {
		verdict = core.VerdictFAIL
	}
	var diags []core.Diagnostic
	// Size budget: warn once when the report overruns auditReportMaxBytes.
	// Diagnostic-only by construction — the verdict is untouched and the
	// artifact on disk is never rewritten (ship SHA-binds it).
	if len(artifact) > auditReportMaxBytes {
		diags = append(diags, core.Diagnostic{
			Severity: "warning",
			Message: fmt.Sprintf("audit-report.md size %d bytes exceeds the %d-byte budget — trim ## Issues to the top findings by severity and keep evidence in the evictable sections (advisory: the verdict and the artifact are unchanged)",
				len(artifact), auditReportMaxBytes),
		})
	}
	// overrodeBy names each gate that ACTUALLY forced verdict=FAIL this call —
	// the single source of truth for "was the narrative overridden?". Keying the
	// conflict record off this list (not off "a gate diagnostic exists") is what
	// keeps the fail-OPEN paths silent: a gate that could not RUN emits a warning
	// and leaves the verdict alone, so it never appears here. Labels are a fixed
	// vocabulary, never offender text — the record is a fingerprint input
	// (failure_digest.go → blocker_breaker.go identical-fingerprint), and the
	// offender detail already rides each gate's own error diagnostic beside it.
	var overrodeBy []string
	overrode := func(gate string) {
		overrodeBy = append(overrodeBy, gate)
		verdict = core.VerdictFAIL
	}
	if reviewErr := validateExplanationReview(artifact, req); reviewErr != nil {
		diags = append(diags, core.Diagnostic{Severity: "error", Message: reviewErr.Error()})
		overrode("explanation documentation qualitative review")
	}

	verdictPath := filepath.Join(req.Workspace, "acs-verdict.json")
	// Probe quarantine runs UNCONDITIONALLY — before the verdict-exists gate.
	// genVerdict is skipped when the auditor pre-wrote acs-verdict.json (the
	// persona instructs exactly that), and a quarantine reachable only through
	// genVerdict would be bypassed with it (review M8). Unconditional also
	// means UNCOUPLED from the genVerdict hook being wired — a config path
	// with no generator must not silently lose the quarantine with it.
	// Degrades open on its own (no worktree / git failure → loud log).
	if qErr := quarantineProbesForRequest(req); qErr != nil {
		diags = append(diags, core.Diagnostic{Severity: "warning",
			Message: fmt.Sprintf("probe quarantine: %s", qErr.Error())})
	}
	// Generate acs-verdict.json when absent and a generator is wired.
	// Pre-staged files (operator/CI) are honored untouched — with one
	// carve-out (cycle-1434): a file STAMPED with a project_root that differs
	// from this phase's own is a foreign-root artifact (minted against the
	// wrong plane's state; its reds/greens describe a different checkout) and
	// is regenerated. Unstamped files keep the full honor — absence means
	// "unstamped", never "mismatch". If generation writes nothing (zero
	// predicates), the missing-file FAIL floor holds.
	if h.genVerdict != nil {
		_, statErr := os.Stat(verdictPath)
		regen := os.IsNotExist(statErr)
		if !regen && statErr == nil {
			if stampedRoot, foreign := foreignRootVerdict(verdictPath, req.ProjectRoot); foreign {
				regen = true
				// Preserve the foreign artifact before regeneration clobbers
				// it (review MEDIUM; the incident class was "the misdiagnosis
				// was invisible from the file") — its reds/greens are the
				// evidence of WHAT the wrong root saw.
				preserved := filepath.Join(req.Workspace, "acs-verdict.foreign.json")
				note := fmt.Sprintf(" (preserved as %s)", filepath.Base(preserved))
				if renameErr := os.Rename(verdictPath, preserved); renameErr != nil {
					note = fmt.Sprintf(" (preserve failed: %v)", renameErr)
				}
				diags = append(diags, core.Diagnostic{
					Severity: "warning",
					Message: fmt.Sprintf("acs-verdict.json was minted under project_root %q, not this phase's %q — foreign-root artifact regenerated (cycle-1434 class)%s",
						stampedRoot, req.ProjectRoot, note),
				})
			}
		}
		if regen {
			if genErr := h.genVerdict(req); genErr != nil {
				diags = append(diags, core.Diagnostic{
					Severity: "warning",
					Message:  fmt.Sprintf("acs-verdict generation failed: %s", genErr.Error()),
				})
			}
		}
	}
	if h.explanationCheck != nil {
		if explanationErr := h.explanationCheck(req); explanationErr != nil {
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("explanation documentation gate: %s", explanationErr.Error()),
			})
			overrode("explanation documentation gate unavailable")
		}
	}

	redCount, redIDs, phantomBindings, shipEligible, acsErr := readACSVerdict(verdictPath)
	// egpsOverride is the one-line reason of whichever EGPS branch forces FAIL —
	// "" means no override happened. Selecting the reason first (instead of
	// appending inside each branch) makes the override single-exit: exactly one
	// gate diagnostic, one `verdict = FAIL`. egpsLabel is that branch's stable
	// name for the conflict record (distinct per branch, so two different EGPS
	// reasons never collapse to one fingerprint).
	// egpsBlocked — NOT `egpsOverride != ""`. The decision to fail a
	// ship-blocking gate must never ride on message FORMATTING: a future message
	// builder that returned "" would silently convert an EGPS red into a
	// non-FAIL (fail-OPEN). The bool is set in the same arm that selects the
	// reason, so the two cannot drift.
	var egpsBlocked bool
	var egpsOverride, egpsLabel string
	switch {
	case acsErr != nil:
		egpsBlocked = true
		egpsOverride = fmt.Sprintf("acs-verdict.json: %s", acsErr.Error())
		egpsLabel = "EGPS acs-verdict.json unreadable"
	case redCount > 0:
		egpsBlocked = true
		egpsOverride = egpsRedMessage(redCount, redIDs, phantomBindings)
		egpsLabel = "EGPS red_count>0"
	case shipEligible != nil && !*shipEligible:
		// The authoritative acssuite SSOT (ship_eligible) can say do-not-ship even
		// when red_count happens to be 0 (a pre-staged/agent-written verdict, or a
		// future acssuite that gates on more than the red count). red_count alone is
		// a proxy; ship_eligible is the ground truth — a narrative PASS must never
		// out-vote it. A verdict that OMITS the field (every verdict written before
		// this cycle, shipEligible==nil) is untouched: back-compat, never a
		// spurious FAIL. This gate is symmetric to the timeout-reconcile path — both
		// route through this same Classify block, no duplicate branch.
		egpsBlocked = true
		egpsOverride = "EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count==0; a narrative PASS cannot override it"
		egpsLabel = "EGPS ship_eligible=false"
	}
	if egpsBlocked {
		diags = append(diags, core.Diagnostic{Severity: "error", Message: egpsOverride})
		overrode(egpsLabel)
	}

	// gofmt CI-parity gate: a cycle whose worktree has any non-gofmt-s-clean
	// .go file would FAIL the CI `vet + fmt` step, so it must FAIL audit here —
	// never ship green-locally/red-in-CI (cycles 339-341). An infra error
	// (gofmt missing, unparseable source) fails OPEN with a loud warning: the
	// gate's own inability to run must not brick the cycle, but is never
	// silently treated as clean.
	if h.gofmtCheck != nil {
		dirty, gerr := h.gofmtCheck(req)
		switch {
		case gerr != nil:
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("gofmt gate skipped (could not run): %s", gerr.Error()),
			})
		case len(dirty) > 0:
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("gofmt: %d file(s) are not gofmt -s clean — CI `vet + fmt` would FAIL. Run `gofmt -w -s .` in go/. Offenders: %s", len(dirty), strings.Join(dirty, ", ")),
			})
			overrode("gofmt")
		}
	}

	// SKILL.md phase-facts drift gate: a cycle that edits .evolve/profiles/*.json
	// (or any SSOT projected into SKILL.md) without regenerating would FAIL the
	// CI TestSkills_NoDrift gate — so it must FAIL audit here (cycle 339 shipped
	// this drift CI-red). Runs in-process (skillcheck.Check); an infra error
	// (e.g. the worktree can't load the registry) fails OPEN with a warning.
	if h.skillsDriftCheck != nil {
		drift, derr := h.skillsDriftCheck(req)
		switch {
		case derr != nil:
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("skills-drift gate skipped (could not run): %s", derr.Error()),
			})
		case len(drift) > 0:
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf("skill projection drift: %d artifact(s) stale vs their SSOTs (SKILL.md phase-facts and/or commands/ stubs) — CI TestSkills_NoDrift would FAIL. Run `evolve skills generate`. Drifted: %s", len(drift), strings.Join(drift, ", ")),
			})
			overrode("skills-drift")
		}
	}

	// CI-parity gates (go vet ./..., -tags acs acs-durable, apicover -enforce):
	// each runs the exact whole-repo CI command against this cycle's worktree.
	// Offenders → FAIL (the cycle would break main CI). Could-not-run → WARN
	// diagnostic, verdict unchanged (fail-open, same as the gofmt/skills gates).
	applyCIGate := func(check func(core.PhaseRequest) ([]string, error), name, failTmpl string) {
		if check == nil {
			return
		}
		switch offenders, cerr := check(req); {
		case cerr != nil:
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("%s skipped (could not run): %s", name, cerr.Error()),
			})
		case len(offenders) > 0:
			diags = append(diags, core.Diagnostic{
				Severity: "error",
				Message:  fmt.Sprintf(failTmpl, len(offenders), strings.Join(offenders, "; ")),
			})
			overrode(name)
		}
	}
	applyCIGate(h.goVetCheck, "go vet gate",
		"go vet ./... reported %d issue(s) — CI `vet + fmt` would FAIL (e.g. import cycle). Offenders: %s")
	applyCIGate(h.acsDurableCheck, "acs-durable gate",
		"acs-durable (-tags acs) FAILED %d check(s) — CI acs-durable gate would FAIL (flag-registry / flag-ceiling / skills-drift). Offenders: %s")
	// The integration-tier finding is the one gate whose local run can diverge
	// from CI's (the requireTmux-guarded tier runs here and skips there), so its
	// template carries a derived parity caveat instead of asserting CI's outcome
	// — see ciparity_caveat.go. Pre-formatting the caveat keeps applyCIGate's
	// (count, offenders) contract unchanged for the other four gates.
	applyCIGate(h.integrationTierCheck, "integration-tier gate",
		integrationTierTemplateWithCaveat(ciParityCaveatNow()))
	applyCIGate(h.apicoverEnforceCheck, "apicover-enforce gate",
		"apicover -enforce flagged %d line(s) in touched enforced packages — CI `api-coverage enforce` would FAIL (unnamed export). Offenders: %s")
	applyCIGate(h.apicoverNewPkgGraduationCheck, "apicover new-package graduation gate",
		"%d new go/internal/<pkg>(s) changed this cycle are absent from .apicover-enforce — the apicover -enforce gate silently skips them (new-package blind spot). Add each to go/.apicover-enforce + an apicover_named_test.go before ship. Offenders: %s")

	// Continuation defect-ledger reconciliation (F1(i)): a lane resuming a
	// FAILed cycle's work must account for every defect that cycle's audit
	// raised. Unaccounted ⇒ FAIL, named by id. A no-op for the ordinary
	// non-continuation cycle (no manifest ⇒ nothing inherited), so the green
	// path is unperturbed.
	ledgerDiags, ledgerBlocked, lineageCycles := reconcileContinuationDefects(req)
	diags = append(diags, ledgerDiags...)
	if ledgerBlocked {
		overrode("continuation defect-ledger")
	}

	// Closure-citation gate (F1 clause 3): the reconcile above governs the
	// LEDGER, but the 1272 laundering happened in PROSE — "the CRITICAL is
	// verified closed", no record named, no cycle blocked. A report that asserts
	// a prior cycle's defect is closed must name the per-defect disposition
	// record on the same line. Invisible to the overwhelming majority of reports,
	// which make no closure claim at all.
	//
	// Demotion, cycle-1502: when the reconcile above RAN against a real lineage
	// and verified every inherited defect against its per-id disposition record,
	// the filing cabinet has provably been opened — a summary line restating
	// those closures without re-citing is a formatting note, not laundering, and
	// forcing FAIL over it produced a verdict-incoherence halt on a 165-green/
	// 0-red cycle. Scoped PER LINE: only claims whose prose cycle refs all fall
	// within the verified lineage (ancestor, its ledger's origin, this cycle)
	// demote — the machine records vouch for exactly those cycles. A claim
	// naming any OTHER cycle, a blocked reconcile, or no lineage at all (the
	// original 1255 shape) forces exactly as before.
	if closureDiags := closureClaimDiagnostics(artifact); len(closureDiags) > 0 {
		vouched := map[int]bool{req.Cycle: true}
		for _, c := range lineageCycles {
			vouched[c] = true
		}
		// closureClaimDiagnostics renders one diagnostic per offender, in
		// order — the offender lines are re-derived here so cycle refs come
		// from the CLAIM text, not the diagnostic boilerplate around it.
		offenders := closureClaimOffenders(artifact)
		forced := false
		for i := range closureDiags {
			// A claim must NAME at least one lineage cycle to demote: an empty
			// ref set means the machine record vouches for nothing on that line
			// — a ref-less "verified closed" is the strong rung's canonical
			// laundering sentence, and demoting it would guard-suppress the one
			// rung the design record forbids suppressing (review BLOCK-1).
			inLineage := false
			if len(lineageCycles) > 0 && i < len(offenders) {
				refs := closureLineCycleRefs(offenders[i])
				inLineage = len(refs) > 0
				for _, ref := range refs {
					if !vouched[ref] {
						inLineage = false
						break
					}
				}
			}
			if inLineage {
				closureDiags[i].Severity = "warning"
				closureDiags[i].Message += " [demoted to advisory: this cycle's defect-ledger reconcile verified every inherited defect of the referenced lineage against its per-id disposition record — cite on the claim line anyway next time]"
			} else {
				forced = true
			}
		}
		diags = append(diags, closureDiags...)
		if forced {
			overrode("closure-claim citation")
		}
	}

	// Auditor-vs-gate coherence record — ONE per Classify call, emitted here so
	// it covers EVERY gate above rather than the EGPS block alone (inbox item
	// `verdict-coherence-auditor-vs-egps`, 4th recurrence: cycles 87/352/456).
	// The gate keeps outranking prose — nothing above is softened — but the
	// disagreement is now durable: error severity IS the wiring, since
	// errorSeverityMessages (core/system_failure.go) keys off Severity=="error",
	// so the record rides the existing AuditFailReasons → <phase>-fail-reason.json
	// → dossier SubstantiveError chain with no new plumbing.
	//
	// core.IsVerdict bounds `narrative` to the four canonical verdicts. It is
	// agent-controlled (the evolve-verdict sentinel's JSON `verdict` field is not
	// enum-checked by ParseVerdictSentinelFull, which rejects only ""), and this
	// string becomes a sha256 failure-fingerprint input: an unbounded value would
	// vary per retry and blind the identical-fingerprint blocker breaker, the same
	// stability invariant egpsRedIDCycleTokens exists to protect. It also keeps
	// the record single-line, so an injected "\n OPERATOR: ..." cannot forge an
	// extra reason line in the dossier or in retro/failure-adapter prompts.
	//
	// BOUNDED is necessary but NOT sufficient (cycle-1127 audit C1): three
	// canonical non-FAIL narratives are reachable, so one recurring defect would
	// still split into three fingerprint buckets against a ceiling of 3. The
	// exact verdict cannot be dropped here — carrying it into the dossier IS
	// this record's purpose, and a warning-severity copy is discarded by
	// errorSeverityMessages — so STABILITY is restored one layer down, where it
	// belongs: core.fingerprint normalizes the `narrative=<verdict>` token out
	// of the hash input while the operator-facing reason keeps it verbatim
	// (display vs identity projections of one source). Keep the literal
	// "narrative=" prefix in the format string — that token is the seam
	// core.normalizeReasonForFingerprint matches on.
	//
	// Silent by construction on every coherent case: no override (fail-OPEN or
	// green gate) ⇒ overrodeBy empty; no parseable/valid narrative ⇒ nothing to
	// disagree with; narrative already FAIL ⇒ auditor and machine agree.
	// The strict_audit WARN→FAIL promotion below is deliberately NOT a conflict:
	// it is a policy re-grade of a verdict the auditor already declined to pass,
	// not a mechanical gate contradicting a clean read.
	if verdictFound && core.IsVerdict(narrative) && narrative != core.VerdictFAIL && len(overrodeBy) > 0 {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			// Neutral by construction: it states WHO said WHAT and stops. The
			// earlier draft asserted the auditor "read the cycle as clean" (false
			// for WARN, meaningless for SKIPPED) and appended a fixed exculpatory
			// "may be a poisoned gate rather than a real regression" sentence —
			// which handed the audited agent a standing lever over the operator's
			// "why did this cycle fail" surface. The reader draws the inference
			// from the evidence beside it.
			Message: verdictConflictMessage(narrative, overrodeBy),
		})
	}

	if verdict == core.VerdictWARN && policy.StrictAuditFor(req.ProjectRoot) {
		verdict = core.VerdictFAIL
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "policy.json workflow.strict_audit promoted WARN to FAIL",
		})
	}

	// A non-empty audit report whose verdict we could not parse, while the EGPS
	// predicate suite is itself green (red_count==0), is almost always a verdict
	// FORMAT miss — not a real defect (cycle-148: the agent wrote
	// "**Verdict: PASS**" but the parser required "## Verdict\n**PASS**", so a
	// genuine PASS was mis-graded FAIL and routed to retro, silently discarding
	// the cycle's work). FAIL loudly so the mis-grade is visible instead of
	// sinking the cycle without a trace.
	if !verdictFound && acsErr == nil && redCount == 0 && strings.TrimSpace(artifact) != "" {
		diags = append(diags, core.Diagnostic{
			Severity: "error",
			Message:  "audit-report.md is non-empty with red_count=0 but declares no parseable verdict — treating as FAIL. Declare it as '## Verdict' + a bold verdict on the next line, or inline as '**Verdict: PASS**'.",
		})
	}

	// Defect-ledger emit (F1(i)), LAST so it sees the final verdict: a rejection
	// leaves an addressable, id-bearing record of what was wrong, which the next
	// continuation is graded against above. Best-effort by design — the ledger
	// is a record OF the rejection, so failing to write it must not change the
	// rejection, but it is never silent.
	//
	// FAIL *and* WARN: a WARN-shipped cycle that still self-reports structured
	// defects would otherwise leave the next continuation nothing to inherit —
	// a laundering channel left open by the mechanism that exists to close
	// laundering. emitDefectLedger mints nothing when the verdict carries no
	// structured defects, so widening the trigger cannot make the reconcile gate
	// vacuous by minting empty ledgers on every warned cycle.
	if verdict == core.VerdictFAIL || verdict == core.VerdictWARN {
		if err := emitDefectLedger(artifact, req); err != nil {
			diags = append(diags, core.Diagnostic{
				Severity: "warning",
				Message:  fmt.Sprintf("defect ledger: could not record this cycle's defects (%s) — a later continuation will have nothing to reconcile against", err.Error()),
			})
		}
	}
	// ADR-0088 chain-of-reasoning, SHADOW stage. Deliberately LAST: the record
	// has to carry the verdict that actually shipped and the gates that forced
	// it, or "the chain agreed with a PASS a gate then force-FAILed" is
	// indistinguishable from "the chain agreed with a PASS that shipped" — the
	// exact question a promotion decision turns on (review MEDIUM). Running it
	// here also means acs-verdict.json has already been generated, so the
	// evidence set is what the auditor could really have read rather than a
	// snapshot taken 40 lines too early (review HIGH).
	//
	// Records only: every value above is computed and returned unchanged.
	recordChainShadow(artifact, req, narrative, string(verdict), overrodeBy)

	return verdict, diags, string(core.PhaseShip)
}

// extractAuditVerdict returns the declared verdict word and whether a
// parseable verdict declaration was found. It tries the canonical
// "## Verdict\n**PASS**" heading first, then common single-line variants. The
// found bool lets the caller distinguish "no verdict declared" (a format miss
// worth a loud diagnostic) from an explicit "FAIL". A real FAIL/WARN/SKIPPED
// declaration is captured verbatim, so broadening the accepted FORMATS never
// turns a real non-PASS verdict into a PASS.
func extractAuditVerdict(content string, stage config.Stage) (string, bool) {
	// Layer-5 strangler: the machine-readable sentinel wins when present; the
	// legacy regex-on-prose remains the fallback for reports written against the
	// older templates.
	if v, ok := phasecontract.ParseVerdictSentinel(content); ok {
		return v, true
	}
	// ADR-0050 §3.10 Slice 5: the regex-on-prose fallbacks serve reports written
	// against older templates; at enforce the sentinel above is mandatory, so gate
	// them off (>= StageEnforce). Below enforce they stay active — byte-identical.
	if stage < config.StageEnforce {
		if v := reportdoc.Verdict(content); v != "" {
			return v, true
		}
	}
	return "", false
}

// readACSVerdict reads the EGPS gate fields from acs-verdict.json. shipEligible
// is a *bool so the caller can distinguish "field absent" (nil — legacy verdicts
// written before ship_eligible existed) from an explicit false (do-not-ship). A
// read/parse error is returned so the missing/malformed-file FAIL floor holds.
// redIDs are the ac_ids of the red results (empty for legacy verdicts without a
// results array) — the diagnostic embeds them so the failure-digest fingerprint
// carries the DEFECT's identity: batch-12 (2026-07-27) halted on the
// identical-fingerprint breaker because three DIFFERENT red predicates all
// produced the byte-identical bare "red_count=1" reason (the cycle-1054/1060
// constant-message collision class, at the gate-block).
// foreignRootVerdict reports whether the verdict at path is STAMPED with a
// project_root different from expected (cycle-1434). Both sides must be
// non-empty — an unstamped file (pre-stamp verdicts, operator pre-stage) or
// an unset phase root can never be "foreign". Unreadable/unparseable files
// return false: the EGPS unreadable branch owns that failure, with its own
// diagnostic.
func foreignRootVerdict(path, expected string) (string, bool) {
	if expected == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var v struct {
		ProjectRoot string `json:"project_root"`
	}
	if json.Unmarshal(data, &v) != nil || v.ProjectRoot == "" {
		return "", false
	}
	if canonRootPath(v.ProjectRoot) == canonRootPath(expected) {
		return v.ProjectRoot, false
	}
	return v.ProjectRoot, true
}

// canonRootPath normalizes a root for comparison, resolving symlinks when the
// path exists (macOS: /var vs /private/var — one side stamped resolved, the
// other not, would otherwise re-run the full single-flight suite every audit;
// the skew can only fire TOWARD regeneration, so this is cost, not
// correctness). A path that fails to resolve falls back to Clean.
func canonRootPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func readACSVerdict(path string) (redCount int, redIDs, phantomBindings []string, shipEligible *bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, nil, nil, fmt.Errorf("read: %w", err)
	}
	var v struct {
		RedCount     int   `json:"red_count"`
		ShipEligible *bool `json:"ship_eligible"`
		Results      []struct {
			ACID            string   `json:"ac_id"`
			Result          string   `json:"result"`
			PhantomBindings []string `json:"phantom_bindings"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return 0, nil, nil, nil, fmt.Errorf("parse: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range v.Results {
		if r.Result != "red" {
			continue
		}
		if r.ACID != "" {
			redIDs = append(redIDs, r.ACID)
		}
		// Phantom names are surfaced so the gate-block can carry the cure
		// (phantom_binding.go in acssuite). Deduped across results: two
		// predicates demanding the same renamed test is one repoint.
		for _, pb := range r.PhantomBindings {
			if pb != "" && !seen[pb] {
				seen[pb] = true
				phantomBindings = append(phantomBindings, pb)
			}
		}
	}
	return v.RedCount, redIDs, phantomBindings, v.ShipEligible, nil
}

// egpsRedIDCycleTokens strips the cycle-numbered chrome from an ACS ac_id:
// "cycle1115/TestC1115_003_BridgeAndRecoveryStayGreen" →
// "BridgeAndRecoveryStayGreen". The SEMANTIC name is the defect's stable
// identity; the cycle-numbered prefix changes on every retry, and embedding it
// would make a real cross-cycle recurrence never collide — blinding the
// identical-fingerprint breaker (the same "never cycle numbers" rule
// verdictFailDistinguisher documents). Both live naming conventions are
// covered by stripping the C-group and the index group INDEPENDENTLY
// (adversarial review caught `C\d+_\d+_` missing the two-part
// "TestC841_Amplify_…"/"TestC416_NEG_…" shape, which left C841_ — a cycle
// number — embedded): three-part TestC<cycle>_<index>_<Name> and two-part
// TestC<cycle>_<Name>. A name that merely starts with 'C'+letters
// ("TestCarryforward_…") is untouched — C\d+ requires digits.
var egpsRedIDCycleTokens = regexp.MustCompile(`^(?:cycle\d+/)?(?:Test)?(?:C\d+_)?(?:\d+_)?`)

// egpsRedMessage renders the one-line EGPS gate-block diagnostic. The red
// predicates' cycle-normalized semantic names (acssuite results order, at most
// maxNamedReds named) make the message — and therefore the failure
// fingerprint — distinct per defect, while the same defect red again on a
// retry cycle (new cycle-numbered ac_id, same semantic name) still collides
// exactly, so the breaker keeps catching real recurrences.
func egpsRedMessage(redCount int, redIDs, phantomBindings []string) string {
	const maxNamedReds = 5
	if len(redIDs) == 0 {
		return fmt.Sprintf("EGPS: red_count=%d (cycle ships only when red_count==0)", redCount) + phantomBindingClause(phantomBindings)
	}
	shown := make([]string, 0, len(redIDs))
	for _, id := range redIDs {
		if n := egpsRedIDCycleTokens.ReplaceAllString(id, ""); n != "" {
			shown = append(shown, n)
		} else {
			shown = append(shown, id) // degenerate id — keep verbatim over dropping
		}
	}
	suffix := ""
	if len(shown) > maxNamedReds {
		suffix = fmt.Sprintf(" +%d more", len(shown)-maxNamedReds)
		shown = shown[:maxNamedReds]
	}
	return fmt.Sprintf("EGPS: red_count=%d [%s%s] (cycle ships only when red_count==0)",
		redCount, strings.Join(shown, " "), suffix) + phantomBindingClause(phantomBindings)
}

// phantomBindingClause renders the actionable half of a phantom-binding red, or
// "" when there are none — the phantom-free message stays byte-identical
// (pinned), so every existing fingerprint and log grep is untouched.
//
// The clause states diagnosis, cure, AND the anti-gaming boundary in the
// directive itself: the reader of this line is usually an agent (retro, a
// continuation builder, a correction round), and an agent told only "test
// missing" reliably reaches for the cheapest green — deleting the predicate —
// which is the exact gaming vector red-on-missing exists to block. The
// 1539-1546 streak burned five cycles on a red this one line would have cured.
func phantomBindingClause(phantomBindings []string) string {
	if len(phantomBindings) == 0 {
		return ""
	}
	return fmt.Sprintf("; PHANTOM binding(s) [%s]: the bound test name does not resolve in its target package (renamed or never created) — repoint the predicate's binding to the real test name or restore the name; do NOT delete the predicate",
		strings.Join(phantomBindings, " "))
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// GenerateVerdict, when set, produces <workspace>/acs-verdict.json from
	// the cycle's ACS predicates if the file is absent (cycle-138/139 fix).
	// nil = no generation (legacy: a pre-staged file is required to PASS).
	// The registry default wires generateACSVerdict (runs acssuite).
	GenerateVerdict func(req core.PhaseRequest) error
	// CheckExplanation binds explanationdocs.Verify into the native Audit
	// override. nil disables only this injected seam (tests); production defaults
	// always wire verifyExplanationDocumentation.
	CheckExplanation func(req core.PhaseRequest) error
	// CheckGofmt, when set, reports the worktree .go files that are not
	// gofmt -s clean; any offender FAILs the audit (CI-parity gate). nil = no
	// gofmt gate. NewDefault wires gofmtCheckDefault.
	CheckGofmt func(req core.PhaseRequest) ([]string, error)
	// CheckSkillsDrift, when set, reports the worktree SKILL.md files whose
	// phase-facts region drifted from its SSOTs; any drift FAILs the audit
	// (CI TestSkills_NoDrift parity). nil = no skills gate. NewDefault wires
	// skillsDriftCheckDefault.
	CheckSkillsDrift func(req core.PhaseRequest) ([]string, error)
	// CheckGoVet / CheckACSDurable / CheckApicoverEnforce are the CI-parity gates
	// (whole-repo go vet ./..., -tags acs acs-durable, apicover -enforce over
	// touched packages); any offender FAILs the audit. nil = no gate.
	// NewDefaultWithStageCompact wires the *Default impls.
	CheckGoVet           func(req core.PhaseRequest) ([]string, error)
	CheckACSDurable      func(req core.PhaseRequest) ([]string, error)
	CheckApicoverEnforce func(req core.PhaseRequest) ([]string, error)
	// CheckIntegrationTier runs the `-tags integration` test tier over the cycle
	// worktree; any offender FAILs the audit (the tier that let TestFleetSoak go
	// CI-red under a green per-cycle audit). nil = no gate.
	// NewDefaultWithStageCompact wires integrationTierCheckDefault.
	CheckIntegrationTier func(req core.PhaseRequest) ([]string, error)
	// CheckApicoverNewPkgGraduation is the new-package graduation gate: it FAILs
	// audit when a changed go/internal/<pkg> is new this cycle and absent from
	// .apicover-enforce (the blind spot apicover -enforce's touched∩enforced
	// scoping drops). nil = no gate. NewDefaultWithStageCompact wires
	// apicoverNewPackageGraduationDefault.
	CheckApicoverNewPkgGraduation func(req core.PhaseRequest) ([]string, error)
	// PhaseIO threads the EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050
	// §3.10 Slice 5). Zero value (StageOff) = byte-identical (prose fallbacks active).
	PhaseIO config.Stage
	// CompactPrompts strips the on-demand reference tail from the disk-loaded agent
	// doc before dispatch. Value flows from workflow.compact_prompts (policy.json);
	// never set to a bare literal here (standing rule: phase-settings-from-config).
	CompactPrompts bool
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks: hooks{
				genVerdict:                    c.GenerateVerdict,
				explanationCheck:              c.CheckExplanation,
				gofmtCheck:                    c.CheckGofmt,
				skillsDriftCheck:              c.CheckSkillsDrift,
				goVetCheck:                    c.CheckGoVet,
				acsDurableCheck:               c.CheckACSDurable,
				integrationTierCheck:          c.CheckIntegrationTier,
				apicoverEnforceCheck:          c.CheckApicoverEnforce,
				apicoverNewPkgGraduationCheck: c.CheckApicoverNewPkgGraduation,
				phaseIO:                       c.PhaseIO,
			},
			Bridge:         c.Bridge,
			Prompts:        c.Prompts,
			NowFn:          c.NowFn,
			CompactPrompts: c.CompactPrompts,
		}),
	}
}

// NewDefault builds the audit phase with production defaults — notably
// GenerateVerdict wired to generateACSVerdict so the EGPS gate auto-generates
// acs-verdict.json when the auditor agent leaves it absent (cycle-138/139 fix).
// BOTH the registry init() and the loop's runner map (go/cmd/evolve/cmd_cycle.go)
// MUST construct audit via this single seam so the generator can never again be
// wired in one phase-construction path but dormant in the other — the
// dual-source divergence that left the loop force-FAILing on a missing verdict
// every cycle (cycle-147). New(Config) stays for tests that pin explicit
// (nil or fake) generators.
func NewDefault(br core.Bridge, prm *prompts.Loader) *Phase {
	return NewDefaultWithStage(br, prm, config.StageOff)
}

// NewDefaultWithStage is NewDefault plus the EVOLVE_PHASE_IO stage (ADR-0050 §3.10
// Slice 5). The composition root (cmd_cycle.go) passes cfg.PhaseIO so the audit
// verdict extraction enforces the sentinel at >= StageEnforce. NewDefault stays as
// the StageOff (byte-identical) convenience for the registry init() and tests.
func NewDefaultWithStage(br core.Bridge, prm *prompts.Loader, stage config.Stage) *Phase {
	return NewDefaultWithStageCompact(br, prm, stage, false)
}

// NewDefaultWithStageCompact is NewDefaultWithStage plus the compact-prompts flag
// (workflow.compact_prompts). Called from cmd_cycle.go with wfCfg.CompactPrompts so
// the reference tail is stripped before dispatch when the policy default is on.
func NewDefaultWithStageCompact(br core.Bridge, prm *prompts.Loader, stage config.Stage, compact bool) *Phase {
	return New(Config{
		Bridge:                        br,
		Prompts:                       prm,
		GenerateVerdict:               generateACSVerdict,
		CheckExplanation:              verifyExplanationDocumentation,
		CheckGofmt:                    gofmtCheckDefault,
		CheckSkillsDrift:              skillsDriftCheckDefault,
		CheckGoVet:                    goVetCheckDefault,
		CheckACSDurable:               acsDurableCheckDefault,
		CheckIntegrationTier:          integrationTierCheckDefault,
		CheckApicoverEnforce:          apicoverEnforceChangedDefault,
		CheckApicoverNewPkgGraduation: apicoverNewPackageGraduationDefault,
		PhaseIO:                       stage,
		CompactPrompts:                compact,
	})
}

// verifyExplanationDocumentation is a native host gate, deliberately separate
// from acs-verdict.json: ACS results are execution-grounded Go predicates, and
// this structural verifier must not impersonate one. Classify turns its error
// into a named deterministic Audit override; Ship independently re-verifies.
func verifyExplanationDocumentation(req core.PhaseRequest) error {
	binding := explanationdocs.CycleBinding{
		ProjectRoot:     req.ProjectRoot,
		Worktree:        req.Worktree,
		Workspace:       req.Workspace,
		BaseSHA:         req.WorktreeBaseSHA,
		Cycle:           req.Cycle,
		RunID:           req.RunID,
		ContractVersion: req.ExplanationDocumentationVersion,
	}
	// The activation belt (single home: explanationdocs, shared with ship —
	// architecture review 2026-09-01). Before this, a dropped/zero
	// ContractVersion silently disabled the whole gate: Verify resolved
	// inactive and audit returned nil. Now a zero version against an ACTIVE
	// host activation, or a live version with no host activation, fails the
	// audit loudly; a nil return below means the host AGREES nothing applies.
	hostActive, beltErr := explanationdocs.CrossCheckActivation(binding)
	if beltErr != nil {
		return beltErr
	}
	if !hostActive {
		return nil
	}
	verified, active, verifyErr := explanationdocs.Verify(context.Background(), binding)
	if verifyErr != nil {
		return verifyErr
	}
	if !active {
		return nil
	}
	if !explanationdocs.SameView(req.BuildExplanation, verified) {
		return fmt.Errorf("typed Build explanation handoff does not match the verified host snapshot")
	}
	return nil
}

// skillsDriftCheckDefault is the production SKILL.md-drift gate: it runs the
// same projection check the CI TestSkills_NoDrift gate runs (skillcheck.Check)
// against the cycle worktree (where the builder's profile/SSOT edits live),
// in-process — no subprocess, so no fork-bomb under `go test`. The worktree is
// preferred; ProjectRoot is the fallback; an empty root is a no-op.
func skillsDriftCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	return skillcheck.Check(root)
}

// gofmtCheckDefault is the production gofmt CI-parity gate: it lists the .go
// files under the cycle worktree's go/ module that are not gofmt -s clean,
// matching CI's `gofmt -d -s .`. The worktree (where the builder wrote this
// cycle's changes) is preferred; ProjectRoot is the fallback. When no go/
// module exists the gate is a no-op (returns nil) rather than an error.
func gofmtCheckDefault(req core.PhaseRequest) ([]string, error) {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return nil, nil
	}
	// ModuleDir is the single source for "where the .go files live", shared with
	// the post-build gofmt normalizer (normalizeBuildGofmt) so the gate and the
	// normalizer can never disagree on which tree to verify vs. format.
	return codequality.UnformattedGoFiles(codequality.ModuleDir(root))
}

// generateACSVerdict runs the ACS predicate suite for req.Cycle and writes
// <workspace>/acs-verdict.json. It runs the predicates discovered under the
// cycle's worktree (where this cycle's acs/cycle-N/*.sh live), falling back
// to the project root. When the suite discovers ZERO predicates it writes
// NOTHING — the audit's missing-file FAIL floor then holds, so a cycle with
// no predicates cannot auto-pass. EvolveDir is derived from the workspace
// (<evolveDir>/runs/cycle-N), matching where audit reads the verdict.
func generateACSVerdict(req core.PhaseRequest) error {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	// Regression test-impact evidence, BEFORE the suite runs. It is about which
	// packages this cycle touched, not about what the suite found, so it must
	// not sit behind the zero-predicate early return below.
	emitTIADecision(req, root)
	// Probe quarantine runs in Classify (before the verdict-exists gate), not
	// here — a pre-staged acs-verdict.json skips this function entirely and
	// must not skip the quarantine with it (review M8).
	// Discover predicate FILES from the worktree (Root), but resolve `.evolve/`
	// runtime data (history, baselines, current build-report) to the MAIN project
	// root via EVOLVE_PROJECT_ROOT — those live in main, not the worktree, so a
	// suite run from the worktree (issue #9 audit-cwd=worktree) would else false-RED
	// every regression predicate that reads .evolve/ (issue #12, cycle-177).
	v, err := acssuite.Run(acssuite.Options{Root: root, ProjectRoot: req.ProjectRoot, Cycle: req.Cycle})
	if err != nil {
		return fmt.Errorf("acssuite run: %w", err)
	}
	if v.PredicateSuite.Total == 0 {
		// No predicates → leave the file absent so the EGPS floor fails the
		// cycle rather than auto-passing it on an empty suite.
		return nil
	}
	// evolveDir = parent of runs/, i.e. dirname(dirname(workspace)).
	evolveDir := filepath.Dir(filepath.Dir(req.Workspace))
	if _, err := acssuite.WriteVerdict(evolveDir, v); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}

// emitTIADecision is the production caller for regression test-impact
// selection: it resolves the staged rollout from .evolve/policy.json, computes
// the decision over this cycle's changed packages, and drops it in the cycle
// workspace as evidence.
//
// Two bounds hold it inside the observability lane. It NEVER changes what
// acssuite runs — at the live "off" default (the checked-in policy.json has no
// regression_tia block) it returns before touching git, so this path is
// byte-identical to its pre-change self. And a failure to write the evidence is
// deliberately swallowed: shadow TIA runs on the path that grades every cycle,
// so a broken evidence sink must degrade quietly rather than turn a healthy
// audit into an error. Observability may never gate the gate.
func emitTIADecision(req core.PhaseRequest, root string) {
	stage := policy.RegressionTIAStageFor(req.ProjectRoot)
	if stage == "off" {
		return
	}
	// An underivable changed set (no repo, git error, concurrent-fleet index
	// lock) must not be read as "nothing changed" — that is the direction that
	// hides a regression class — so it degrades to an empty scope, which the
	// selector treats as UNKNOWN impact and skips nothing for.
	changed, derivable := changedpkgs.FromGitChecked(root, "HEAD")
	if !derivable {
		changed = nil
	}
	d := regressiontia.Compute(stage, root, codequality.ModuleDir(root), changed)
	if d.Stage == "" {
		return
	}
	_, _ = regressiontia.Emit(req.Workspace, d)
}

// verdictConflictMessage renders the verdict-conflict record — the one place
// its format lives. Consumers key on the prefix: core.normalizeReasonForFingerprint
// stabilizes the `narrative=<verdict>` token, and the bookkeeping-regrade
// classifier (core.BookkeepingConflictAuditReason) matches the record's head;
// bookkeeping_reason_singlesource_test.go pins this producer to that matcher.
func verdictConflictMessage(narrative string, overrodeBy []string) string {
	return fmt.Sprintf("verdict-conflict: auditor narrative=%s but %d deterministic gate(s) forced FAIL [%s] — "+
		"the gate outranks the narrative (ship policy unchanged); both readings are recorded so the "+
		"disagreement is weighable. Gate detail is in the error diagnostics beside this one.",
		narrative, len(overrodeBy), strings.Join(overrodeBy, ", "))
}

func init() {
	registry.Register(string(core.PhaseAudit), func(req core.PhaseRequest) core.PhaseRunner {
		return NewDefault(bridge.NewDefault(req.ProjectRoot), prompts.NewForProject(req.ProjectRoot))
	})
}

// recordChainShadow writes the ADR-0088 chain-versus-narrative comparison into
// the phase workspace. Best-effort and silent on failure by design: a shadow
// measurement must never influence, delay, or brick the decision it is
// measuring — the same posture the bad_verdict instrumentation ships with.
//
// The evidence set is what is actually ON DISK in the workspace, not what the
// prompt claimed: the question the record answers is whether the judge COULD
// have walked the chain, and a file the dispatch mentioned but never produced
// would make that answer a lie.
func recordChainShadow(artifact string, req core.PhaseRequest, narrative, shipped string, overrodeBy []string) {
	if req.Workspace == "" {
		return
	}
	var given []string
	for _, a := range auditchain.RequiredEvidence(string(core.PhaseAudit)) {
		if _, err := os.Stat(filepath.Join(req.Workspace, a)); err == nil {
			given = append(given, a)
		}
	}
	rec := auditchain.Shadow(req.Cycle, string(core.PhaseAudit), artifact, narrative, given)
	rec.ShippedVerdict = shipped
	rec.OverrodeBy = overrodeBy
	_ = atomicwrite.JSON(filepath.Join(req.Workspace, auditchain.ShadowRecordFile), rec)
}
