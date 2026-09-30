package specrunner

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const (
	SentinelStageOff     = ""
	SentinelStageShadow  = "shadow"
	SentinelStageEnforce = "enforce"
)

const verdictShadowRecordPrefix = "judgment-verdict-shadow"

func portableFilenameRune(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		return r
	default:
		return '_'
	}
}

func VerdictShadowRecordFile(phase string) string {
	safe := strings.Map(portableFilenameRune, phase)
	if safe == "" {
		safe = "unnamed"
	}
	return verdictShadowRecordPrefix + "-" + safe + ".json"
}

type VerdictShadowRecord struct {
	Cycle             int    `json:"cycle"`
	Phase             string `json:"phase"`
	Stage             string `json:"stage"`
	StructuralVerdict string `json:"structural_verdict"`
	SentinelConsulted bool   `json:"sentinel_consulted"`
	SentinelPresent   bool   `json:"sentinel_present"`
	SentinelVerdict   string `json:"sentinel_verdict,omitempty"`
	EffectiveVerdict  string `json:"effective_verdict"`
	WouldFlip         bool   `json:"would_flip"`
	Rationale         string `json:"rationale"`
}

func applySentinelStage(o classifyOutcome, artifact, stage string) classifyOutcome {
	switch stage {
	case SentinelStageOff:
		return o
	case SentinelStageShadow, SentinelStageEnforce:
	default:
		return structuralOnly(core.VerdictFAIL, append(o.diags, core.Diagnostic{
			Severity: "error",
			Message: fmt.Sprintf("invalid verdict_from_sentinel %q: must be %q (off), %q or %q",
				stage, SentinelStageOff, SentinelStageShadow, SentinelStageEnforce),
		}))
	}
	o.sentinelConsulted = true

	stated, ok := phasecontract.ParseVerdictSentinel(artifact)
	switch {
	case !ok:
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message:  "verdict_from_sentinel: no readable verdict sentinel — keeping the structural verdict (fail-open)",
		})
		return o
	case !core.IsVerdict(stated):
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message: fmt.Sprintf("verdict_from_sentinel: stated verdict %q is not PASS/FAIL/WARN/SKIPPED — keeping the structural verdict (fail-open)",
				stated),
		})
		return o
	}

	o.sentinelVerdict, o.sentinelPresent = stated, true
	if stage == SentinelStageEnforce {
		o.effectiveVerdict = stated
		return o
	}
	if stated != o.effectiveVerdict {
		o.diags = append(o.diags, core.Diagnostic{
			Severity: "warn",
			Message: fmt.Sprintf("verdict_from_sentinel=shadow: phase stated %s, cycle routed %s — recorded, not enforced",
				stated, o.effectiveVerdict),
		})
	}
	return o
}

func classifyShadow(cycle int, phase, artifact string, rules *phasespec.ClassifyRules) (VerdictShadowRecord, bool) {
	if rules == nil {
		return VerdictShadowRecord{}, false
	}
	return shadowRecord(cycle, phase, rules.VerdictFromSentinel, evaluate(artifact, rules))
}

func shadowRecord(cycle int, phase, stage string, o classifyOutcome) (VerdictShadowRecord, bool) {
	if stage == SentinelStageOff {
		return VerdictShadowRecord{}, false
	}
	rec := VerdictShadowRecord{
		Cycle:             cycle,
		Phase:             phase,
		Stage:             stage,
		StructuralVerdict: o.structuralVerdict,
		SentinelConsulted: o.sentinelConsulted,
		SentinelPresent:   o.sentinelPresent,
		SentinelVerdict:   o.sentinelVerdict,
		EffectiveVerdict:  o.effectiveVerdict,
		WouldFlip:         o.sentinelPresent && o.sentinelVerdict != o.effectiveVerdict,
	}
	rec.Rationale = shadowRationale(rec)
	return rec, true
}

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

func writeVerdictShadow(workspace string, rec VerdictShadowRecord, optedIn bool) error {
	if !optedIn || workspace == "" {
		return nil
	}
	return atomicwrite.JSON(filepath.Join(workspace, VerdictShadowRecordFile(rec.Phase)), rec)
}
