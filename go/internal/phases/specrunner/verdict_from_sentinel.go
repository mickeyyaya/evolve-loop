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
// The signal was never missing. Measured across this repo's entire run history,
// 225 of 225 premise-challenge/adversarial-review reports carry a well-formed
// sentinel. Nothing read them. So this is a WIRING fix, and deliberately not a
// new grammar: it reuses phasecontract's sentinel parser — the same one the
// contract gate and the verdict cache already read — rather than inventing a
// second way to say "FAIL" that could drift from the first.
//
// Why a rollout stage and not a switch. The same measurement says the
// population is uncalibrated: 100 of those 225 state FAIL, and
// premise-challenge alone states FAIL on 52 of 55 reports — 20 of 20 since
// cycle-1500. A verdict emitted for years into a void does not get corrected,
// because nothing ever contradicted it; turning it authoritative in one step
// would halt nearly every cycle at that phase. Shadow measures the population
// first. That is this repo's most expensive recurring habit, and the one place
// it is cheapest to break.

import (
	"fmt"
	"path/filepath"

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

// VerdictShadowRecordFile is the per-cycle measurement, written into the phase
// workspace beside the report it is about — the same placement as
// auditchain.ShadowRecordFile so one soak sweep can read both.
const VerdictShadowRecordFile = "judgment-verdict-shadow.json"

// VerdictShadowRecord is one phase's stated-versus-routed comparison.
//
// Every field answers a question the promotion decision actually turns on: what
// the phase said, what the cycle did, whether those differed, and — when they
// did not — whether that was agreement or merely an unreadable sentinel.
type VerdictShadowRecord struct {
	Cycle int    `json:"cycle"`
	Phase string `json:"phase"`
	Stage string `json:"stage"`
	// StructuralVerdict is the legacy reading: what the cycle would carry with
	// none of this wired. Kept explicitly so a record read after a promotion
	// still shows what was being suppressed before it.
	StructuralVerdict string `json:"structural_verdict"`
	// SentinelPresent separates "the agent stated nothing readable" from "the
	// agent stated PASS". Conflating them would let a population of malformed
	// reports masquerade as a population of clean ones — and the flip rate
	// computed from that is the number an operator would promote on.
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
func applySentinelStage(o classifyOutcome, artifact string, rules *phasespec.ClassifyRules) classifyOutcome {
	o.stage = rules.VerdictFromSentinel
	switch o.stage {
	case SentinelStageOff:
		return o
	case SentinelStageShadow, SentinelStageEnforce:
	default:
		// A typo'd stage silently disables the gate, and an inert gate must
		// fail LOUDLY — the cycle-241 declared-semantics rule this classifier
		// already applies to fail_if_signal and verdict_on_pass. Failing here
		// costs one cycle; passing silently costs however long nobody notices.
		return structuralOnly(core.VerdictFAIL, []core.Diagnostic{{
			Severity: "error",
			Message: fmt.Sprintf("invalid verdict_from_sentinel %q: must be %q (off), %q or %q",
				o.stage, SentinelStageOff, SentinelStageShadow, SentinelStageEnforce),
		}})
	}

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
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message: fmt.Sprintf("verdict_from_sentinel: stated verdict %q is not PASS/FAIL/WARN/SKIPPED — keeping the structural verdict (fail-open)",
				stated),
		})
		return o
	}

	o.sentinel, o.present = stated, true
	if o.stage == SentinelStageEnforce {
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

// ClassifyShadow builds the measurement record for one phase's artifact.
// ok is false when the phase never opted in, so a caller can write the record
// unconditionally without a phase-name literal anywhere in Go.
//
// Pure, like EvaluateClassify: it decides WHAT to record, never where it lands.
func ClassifyShadow(cycle int, phase, artifact string, rules *phasespec.ClassifyRules) (VerdictShadowRecord, bool) {
	if rules == nil || rules.VerdictFromSentinel == SentinelStageOff {
		return VerdictShadowRecord{}, false
	}
	o := evaluate(artifact, rules)
	rec := VerdictShadowRecord{
		Cycle:             cycle,
		Phase:             phase,
		Stage:             rules.VerdictFromSentinel,
		StructuralVerdict: o.structural,
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

// writeVerdictShadow persists the record beside the artifacts it describes.
// Best-effort and silent on failure by design, the same posture as the
// audit-chain shadow: a measurement must never influence, delay, or brick the
// decision it is measuring.
func writeVerdictShadow(workspace string, rec VerdictShadowRecord, ok bool) {
	if !ok || workspace == "" {
		return
	}
	_ = atomicwrite.JSON(filepath.Join(workspace, VerdictShadowRecordFile), rec)
}
