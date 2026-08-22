package specrunner

// verdict_from_sentinel.go — letting a judgment phase's OWN verdict decide.
//
// The defect: EvaluateClassify judged a spec-driven phase from STRUCTURE ONLY.
// Sections present and non-empty meant PASS, so a phase whose entire job is to
// render judgment could conclude "FAIL (BLOCK). The cycle must not proceed as
// framed" — and emit the canonical machine sentinel saying exactly that — while
// the orchestrator classified it PASS and ran the cycle to completion
// (cycle-1528, whose ignored objection was correct: the redesign it forced
// shipped as ADR-0090). The loop paid for a full agent dispatch every cycle and
// discarded its conclusion.
//
// The signal was never missing — every judgment report on disk carries a
// well-formed sentinel. So this is a WIRING fix, and deliberately not a new
// grammar: it reuses phasecontract's sentinel parser — the same one the
// contract gate and the verdict cache already read — rather than inventing a
// second way to say "FAIL" that could drift from the first.
//
// Why a rollout stage and not a switch: the population is UNCALIBRATED. A
// verdict emitted for years into a void does not get corrected, because nothing
// ever contradicted it, and turning it authoritative in one step would halt
// nearly every cycle at that phase. Shadow measures the population first. The
// measured counts live in ADR-0091 (docs/architecture/adr/0091-*) and are
// deliberately NOT repeated here — they move every cycle, and a stale number in
// a comment is worse than a pointer to the place that owns it.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// Rollout stages for ClassifyRules.VerdictFromSentinel.
const (
	// SentinelStageOff is the absent key: structure decides and the stated
	// verdict is discarded. Byte-identical to the pre-fix classifier for every
	// phase that does not opt in.
	SentinelStageOff = ""
	// SentinelStageShadow routes exactly as off does, and records what the
	// stated verdict WOULD have made it.
	SentinelStageShadow = "shadow"
	// SentinelStageEnforce makes the agent's stated verdict authoritative.
	SentinelStageEnforce = "enforce"
)

// verdictShadowRecordPrefix names the per-cycle measurement written into the
// phase workspace, beside the report it is about.
//
// The filename is PHASE-SCOPED, and that is load-bearing rather than cosmetic.
// The workspace is core.RunWorkspacePath(root, cycle) — one directory PER CYCLE,
// shared by every phase — and both opted-in judgment phases routinely run in the
// same cycle (premise-challenge after triage, adversarial-review after build:
// 47 of 55 premise-challenge cycles in the live tree also ran adversarial-review).
// A single shared filename would let the later phase silently overwrite the
// earlier one, destroying ~85% of premise-challenge's samples — and destroying
// them with a BIAS, since the lost cycles are exactly the ones that also
// produced a build. The auditchain.ShadowRecordFile precedent this otherwise
// mirrors is a bare constant only because `audit` runs once per cycle; that
// uniqueness assumption does not survive a per-phase key.
const verdictShadowRecordPrefix = "judgment-verdict-shadow"

// VerdictShadowRecordFile is the record's filename for one phase.
// Non-portable characters are folded to '_' so a phase name can never traverse
// out of the workspace or collide with a sibling by accident.
func VerdictShadowRecordFile(phase string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, phase)
	if safe == "" {
		safe = "unnamed"
	}
	return verdictShadowRecordPrefix + "-" + safe + ".json"
}

// VerdictShadowRecord is one phase's stated-versus-routed comparison.
//
// Every field answers a question the promotion decision actually turns on: what
// the phase said, what the cycle did, whether those differed, and — when they
// did not — whether that was agreement, an unreadable sentinel, or a sentinel
// nobody ever looked at.
type VerdictShadowRecord struct {
	Cycle int    `json:"cycle"`
	Phase string `json:"phase"`
	Stage string `json:"stage"`
	// StructuralVerdict is the legacy reading: what the cycle would carry with
	// none of this wired. Kept explicitly so a record read after a promotion
	// still shows what was being suppressed before it.
	StructuralVerdict string `json:"structural_verdict"`
	// SentinelConsulted distinguishes "we looked and found nothing readable"
	// from "we never looked". The stage runs LAST, so a structurally broken
	// artifact — or a typo'd stage word — short-circuits before the parse, and
	// without this bit such a record was indistinguishable from a malformed
	// report. That silently under-reports malformedness in exactly the cycles
	// where the phase misbehaved, which is the population an operator would
	// promote on.
	SentinelConsulted bool `json:"sentinel_consulted"`
	// SentinelPresent separates "the agent stated nothing readable" from "the
	// agent stated PASS". Meaningful only when SentinelConsulted is true.
	SentinelPresent bool   `json:"sentinel_present"`
	SentinelVerdict string `json:"sentinel_verdict,omitempty"`
	// EffectiveVerdict is what the cycle actually carried out of this phase.
	EffectiveVerdict string `json:"effective_verdict"`
	// WouldFlip is the datum the soak exists to count: the phase stated
	// something and the cycle routed otherwise. False under enforce by
	// construction — there the stated verdict IS the routed one.
	WouldFlip bool   `json:"would_flip"`
	Rationale string `json:"rationale"`
}

// applySentinelStage resolves the stated verdict against the configured stage.
// It runs only after every structural check has passed, so a stated verdict can
// never launder a malformed artifact past the section requirement.
func applySentinelStage(o classifyOutcome, artifact, stage string) classifyOutcome {
	switch stage {
	case SentinelStageOff:
		return o
	case SentinelStageShadow, SentinelStageEnforce:
	default:
		// A typo'd stage silently disables the gate, and an inert gate must
		// fail LOUDLY — the cycle-241 declared-semantics rule this classifier
		// already applies to fail_if_signal and verdict_on_pass. Failing here
		// costs one cycle; passing silently costs however long nobody notices.
		return structuralOnly(core.VerdictFAIL, append(o.diags, core.Diagnostic{
			Severity: "error",
			Message: fmt.Sprintf("invalid verdict_from_sentinel %q: must be %q (off), %q or %q",
				stage, SentinelStageOff, SentinelStageShadow, SentinelStageEnforce),
		}))
	}
	o.consulted = true

	stated, ok := phasecontract.ParseVerdictSentinel(artifact)
	switch {
	case !ok:
		// FAIL-OPEN. An absent or malformed sentinel keeps today's verdict, so
		// a report the parser cannot read never hard-blocks a cycle. Only an
		// unambiguous stated verdict moves anything.
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message:  "verdict_from_sentinel: no readable verdict sentinel — keeping the structural verdict (fail-open)",
		})
		return o
	case !core.IsVerdict(stated):
		// Readable but not a verdict this system has: same fail-open, said out
		// loud, because a silently discarded conclusion is the whole defect.
		// Note core.IsVerdict is case-SENSITIVE, so a lowercased "fail" lands
		// here rather than being honored.
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message: fmt.Sprintf("verdict_from_sentinel: stated verdict %q is not PASS/FAIL/WARN/SKIPPED — keeping the structural verdict (fail-open)",
				stated),
		})
		return o
	}

	o.sentinel, o.present = stated, true
	if stage == SentinelStageEnforce {
		o.effective = stated
		return o
	}
	if stated != o.effective {
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message: fmt.Sprintf("verdict_from_sentinel=shadow: phase stated %s, cycle routed %s — recorded, not enforced",
				stated, o.effective),
		})
	}
	return o
}

// classifyShadow builds the measurement record for one phase's artifact.
// ok is false when the phase never opted in, so a caller can build the record
// unconditionally without a phase-name literal anywhere in Go.
func classifyShadow(cycle int, phase, artifact string, rules *phasespec.ClassifyRules) (VerdictShadowRecord, bool) {
	if rules == nil {
		return VerdictShadowRecord{}, false
	}
	return shadowRecord(cycle, phase, rules.VerdictFromSentinel, evaluate(artifact, rules))
}

// shadowRecord projects an already-computed outcome into the record. Split from
// classifyShadow so the live path (hooks.Classify) evaluates ONCE and reads both
// views off that single pass, instead of classifying the same artifact twice.
func shadowRecord(cycle int, phase, stage string, o classifyOutcome) (VerdictShadowRecord, bool) {
	if stage == SentinelStageOff {
		return VerdictShadowRecord{}, false
	}
	rec := VerdictShadowRecord{
		Cycle:             cycle,
		Phase:             phase,
		Stage:             stage,
		StructuralVerdict: o.structural,
		SentinelConsulted: o.consulted,
		SentinelPresent:   o.present,
		SentinelVerdict:   o.sentinel,
		EffectiveVerdict:  o.effective,
		WouldFlip:         o.present && o.sentinel != o.effective,
	}
	rec.Rationale = shadowRationale(rec)
	return rec, true
}

// shadowRationale states the record's meaning in the record itself, so a soak
// sweep does not have to re-derive it from the fields — and so a human reading
// one file understands it without this package open beside them.
func shadowRationale(rec VerdictShadowRecord) string {
	switch {
	case !rec.SentinelConsulted && !knownSentinelStage(rec.Stage):
		return fmt.Sprintf("invalid verdict_from_sentinel stage %q — the phase was failed on its own config, not on its artifact", rec.Stage)
	case !rec.SentinelConsulted:
		return fmt.Sprintf("structural verdict %s decided before the stated verdict was consulted; the report's own sentinel was never read", rec.StructuralVerdict)
	case !rec.SentinelPresent:
		return "no readable verdict sentinel; structural verdict kept (fail-open)"
	case rec.WouldFlip:
		return fmt.Sprintf("phase stated %s, cycle routed %s — enforcing this stage would have changed the cycle",
			rec.SentinelVerdict, rec.EffectiveVerdict)
	case rec.Stage == SentinelStageEnforce:
		return fmt.Sprintf("stated %s is authoritative at enforce", rec.SentinelVerdict)
	default:
		return fmt.Sprintf("phase stated %s and the cycle routed the same", rec.SentinelVerdict)
	}
}

func knownSentinelStage(s string) bool {
	return s == SentinelStageOff || s == SentinelStageShadow || s == SentinelStageEnforce
}

// writeVerdictShadow persists the record beside the artifacts it describes.
// Best-effort: a measurement must never influence, delay, or brick the decision
// it is measuring — the same posture as the audit-chain shadow. It is NOT
// silent, though: a returned error becomes a warn diagnostic at the call site,
// because a permanently failing write yields an empty soak with zero signal and
// nothing else would ever say so. Diagnostics do not affect routing, so saying
// it out loud cannot perturb the decision being measured.
func writeVerdictShadow(workspace string, rec VerdictShadowRecord, ok bool) error {
	if !ok || workspace == "" {
		return nil
	}
	return atomicwrite.JSON(filepath.Join(workspace, VerdictShadowRecordFile(rec.Phase)), rec)
}
