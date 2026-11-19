package config

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const (
	CodeUnknownValue       signalcenter.Code = "CONFIG_UNKNOWN_VALUE"
	CodeWeakSpine          signalcenter.Code = "CONFIG_WEAK_SPINE"
	CodeSpineOrder         signalcenter.Code = "CONFIG_SPINE_ORDER"
	CodeInertPhaseEnable   signalcenter.Code = "CONFIG_INERT_PHASE_ENABLE"
	CodeRegistryUnreadable signalcenter.Code = "CONFIG_REGISTRY_UNREADABLE"
	CodeRegistryMalformed  signalcenter.Code = "CONFIG_REGISTRY_MALFORMED"
)

const (
	codeUnknownValue       = "unknown-value"
	codeWeakSpine          = "weak-spine"
	codeSpineOrder         = "spine-order"
	codeInertPhaseEnable   = "inert-phase-enable"
	codeRegistryUnreadable = "registry-unreadable"
	codeRegistryMalformed  = "registry-malformed"
)

var signalCodes = map[string]signalcenter.Code{
	codeUnknownValue:       CodeUnknownValue,
	codeWeakSpine:          CodeWeakSpine,
	codeSpineOrder:         CodeSpineOrder,
	codeInertPhaseEnable:   CodeInertPhaseEnable,
	codeRegistryUnreadable: CodeRegistryUnreadable,
	codeRegistryMalformed:  CodeRegistryMalformed,
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeUnknownValue, "a routing dial value is outside its closed vocabulary, or a registry contract is incomplete. The documented fail-safe ran (never a kill-path enable: stage → off, routing_mode → llm, model_routing → static, enabled → content). Bad conditional rules or max_optional_insertions are ignored. fields.step names the step (registry or policy). Other fields: fields.key, fields.value, fields.default")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeWeakSpine, "the mandatory_phases of the phase registry omit audit or ship. The audit-before-ship guarantee rests on the legality graph and audit verdict branch alone. The cycle proceeds. fields.step=spine, fields.missing = audit, ship or audit+ship")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeSpineOrder, "the registry's phases[] order places ship before audit. The artifact-backed floor cannot gate ship on a shippable audit by position. The legality graph and audit verdict branch still block it. fields.step=spine, audit_pos, ship_pos")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeInertPhaseEnable, "a phase is force-enabled (enabled: on) while dynamic_routing is below advisory and it is not mandatory or in the static state machine. The enable never runs it (cycle-120 confusion). Set dynamic_routing>=advisory or remove the enable. fields.step=inert, phase, stage")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRegistryUnreadable, "docs/architecture/phase-registry.json exists, but the reader failed to read it (permissions or directory; absence is silent). Every cycle runs on the compiled baseline. The baseline omits triage, registry order, enabled/routing blocks, goal recipes, and deliverable kinds. fields.step=registry, path, err")
	signalcenter.RegisterCode(signalcenter.ModuleConfig, CodeRegistryMalformed, "docs/architecture/phase-registry.json was read, but it is not valid JSON (for example, a trailing comma). It triggers the same compiled-baseline degrade as CONFIG_REGISTRY_UNREADABLE (no triage, no registry order). The reason holds the decoder error. fields.step=registry, path, err")
}

func warn(ws *[]Warning, code, msg string, fields map[string]string) {
	own := make(map[string]string, len(fields)+4)
	for k, v := range fields {
		own[k] = v
	}
	*ws = append(*ws, Warning{Code: code, Message: msg, Fields: own})
}

func stamp(ws []Warning, kv ...string) {
	for i := range ws {
		for j := 0; j+1 < len(kv); j += 2 {
			ws[i].Fields[kv[j]] = kv[j+1]
		}
	}
}

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

func IsRegistryFault(w Warning) bool {
	switch w.Code {
	case codeRegistryUnreadable, codeRegistryMalformed, codeWeakSpine:
		return true
	}
	return false
}
