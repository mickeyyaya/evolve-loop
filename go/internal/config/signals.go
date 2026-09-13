package config

// signals.go — what module config can say: the six registered codes, the ONE
// projection from the legacy Warning.Code vocabulary onto them, the one
// appender every producer site calls, the range stamper the pipeline uses to
// attach the triage fields, and the one producer that turns a Warning into a
// config.warning event.

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

// The unit's codes — registered in init with the reasons a triage reads.
const (
	CodeUnknownValue       signalcenter.Code = "CONFIG_UNKNOWN_VALUE"
	CodeWeakSpine          signalcenter.Code = "CONFIG_WEAK_SPINE"
	CodeSpineOrder         signalcenter.Code = "CONFIG_SPINE_ORDER"
	CodeInertPhaseEnable   signalcenter.Code = "CONFIG_INERT_PHASE_ENABLE"
	CodeRegistryUnreadable signalcenter.Code = "CONFIG_REGISTRY_UNREADABLE"
	CodeRegistryMalformed  signalcenter.Code = "CONFIG_REGISTRY_MALFORMED"
)

// The legacy Warning.Code vocabulary the in-package pins and `evolve solution`
// read — the ONE spelling each producer site passes to warn.
const (
	codeUnknownValue       = "unknown-value"
	codeWeakSpine          = "weak-spine"
	codeSpineOrder         = "spine-order"
	codeInertPhaseEnable   = "inert-phase-enable"
	codeRegistryUnreadable = "registry-unreadable"
	codeRegistryMalformed  = "registry-malformed"
)

// signalCodes is the ONE projection from a Warning.Code onto its registered
// signal code — six explicit pairs (data, not a string transform), walked by
// TestSignalCodes_TableIsTheOneProjectionWithNoPhantoms. emit reads it; a Code
// absent from it would reach the Center empty and be stamped
// SIGNALCENTER_MISSING_CODE (raised, never dropped), which the stream golden
// asserts never happens.
var signalCodes = map[string]signalcenter.Code{
	codeUnknownValue:       CodeUnknownValue,
	codeWeakSpine:          CodeWeakSpine,
	codeSpineOrder:         CodeSpineOrder,
	codeInertPhaseEnable:   CodeInertPhaseEnable,
	codeRegistryUnreadable: CodeRegistryUnreadable,
	codeRegistryMalformed:  CodeRegistryMalformed,
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeUnknownValue, "a routing dial's value is outside its closed vocabulary, or a registry contract is incomplete, and the documented fail-safe was taken — never a kill-path enable (a stage word → off, routing_mode → llm, model_routing → static, enabled → content, EVOLVE_SANDBOX → the current mode, a bad conditional rule or max_optional_insertions ignored); fields.step names the resolving step (registry, env or policy), fields.key the dial, fields.value the word, fields.default the fallback")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeWeakSpine, "mandatory_phases (the registry's or EVOLVE_MANDATORY_PHASES) omits audit and/or ship, so the audit-before-ship guarantee rests on the legality graph and the audit verdict branch alone; the cycle proceeds; fields.step=spine, fields.missing = audit, ship or audit+ship")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeSpineOrder, "the registry's phases[] order places ship before audit, so the artifact-backed floor cannot gate ship on a shippable audit by position; the legality graph and the audit verdict branch still block it; fields.step=spine, audit_pos, ship_pos")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeInertPhaseEnable, "a phase is force-enabled (enabled: on) while dynamic_routing is below advisory and it is neither mandatory nor in the static state machine, so the enable never runs it (the cycle-120 confusion); set dynamic_routing>=advisory or remove the enable; fields.step=inert, phase, stage")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRegistryUnreadable, "docs/architecture/phase-registry.json exists but could not be read (permissions, a directory at the path — absence is silent); every cycle runs on the compiled baseline, which omits triage, the registry order, the enabled/routing blocks, the goal recipes and the deliverable kinds; fields.step=registry, path, err")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRegistryMalformed, "docs/architecture/phase-registry.json was read but is not valid JSON (a trailing comma is the classic); the same compiled-baseline degrade as CONFIG_REGISTRY_UNREADABLE — no triage, no registry order — with the decoder's error in the reason; fields.step=registry, path, err")
}

// warn is the ONE appender: every Warning the package mints passes through it
// with its code, its operator sentence and the fields the producer knows. The
// Fields map is always the Warning's own (copied), so the range stamper can
// add the step/source/key/path the caller knows without touching the
// producer's map.
func warn(ws *[]Warning, code, msg string, fields map[string]string) {
	own := make(map[string]string, len(fields)+4)
	for k, v := range fields {
		own[k] = v
	}
	*ws = append(*ws, Warning{Code: code, Message: msg, Fields: own})
}

// stamp sets each key/value pair of kv on every Warning in the range — the
// pipeline stamps what only it knows (the step, the source, the registry path,
// the env var, the phase name) onto the warnings a step appended.
func stamp(ws []Warning, kv ...string) {
	for i := range ws {
		for j := 0; j+1 < len(kv); j += 2 {
			ws[i].Fields[kv[j]] = kv[j+1]
		}
	}
}

// emit is the ONE producer: each Warning becomes a config.warning WARN under
// module config, Cycle 0 (the loader is batch-level: it runs before any cycle
// number exists, so the root's durable sink files it under <evolveDir>),
// Origin naming the exported method, the Message as the reason and the
// Warning's fields verbatim. A nil Center is the Null Object.
func (l *Loader) emit(origin string, ws []Warning) {
	for _, w := range ws {
		l.center().Emit(signalcenter.Event{
			Module: signalcenter.ModuleConfig, Origin: origin, Kind: signalcenter.KindConfigWarning,
			Severity: signalcenter.SeverityWarn, Code: signalCodes[w.Code], Reason: w.Message, Fields: w.Fields,
		})
	}
}

func (l *Loader) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}
