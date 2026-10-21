package router

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func evalCondition(sig RoutingSignals, c config.Condition) bool {
	num, isNum, str, isPresent := resolveField(sig, c.Field)
	if !isPresent {
		return false
	}
	switch c.Op {
	case "eq", "==":
		if isNum {
			if v, ok := coerceNum(c.Field, c.Value); ok {
				return num == v
			}
		}
		return str == coerceStr(c.Value)
	case "ne", "!=":
		if isNum {
			if v, ok := coerceNum(c.Field, c.Value); ok {
				return num != v
			}
		}
		return str != coerceStr(c.Value)
	case "gt", ">":
		v, ok := coerceNum(c.Field, c.Value)
		return isNum && ok && num > v
	case "gte", ">=":
		v, ok := coerceNum(c.Field, c.Value)
		return isNum && ok && num >= v
	case "lt", "<":
		v, ok := coerceNum(c.Field, c.Value)
		return isNum && ok && num < v
	case "lte", "<=":
		v, ok := coerceNum(c.Field, c.Value)
		return isNum && ok && num <= v
	default:
		return false
	}
}

func evalCondRule(sig RoutingSignals, r config.CondRule) bool {
	for _, c := range r.Clauses() {
		if !evalCondition(sig, config.Condition{Field: c.Field, Op: c.Op, Value: c.Value}) {
			return false
		}
	}
	return true
}

type fieldResolver func(sig RoutingSignals, field string) (float64, bool, string, bool)

func textField(get func(RoutingSignals) string) fieldResolver {
	return func(sig RoutingSignals, _ string) (float64, bool, string, bool) { return 0, false, get(sig), true }
}

func numberField(get func(RoutingSignals) float64) fieldResolver {
	return func(sig RoutingSignals, _ string) (float64, bool, string, bool) { return get(sig), true, "", true }
}

func typedOrGenericField(get func(RoutingSignals) string) fieldResolver {
	return func(sig RoutingSignals, field string) (float64, bool, string, bool) {
		return resolveTypedOrGeneric(sig, field, get(sig))
	}
}

var typedFields = map[string]fieldResolver{
	"cycle_size":                  textField(RoutingSignals.CycleSize),
	"triage.cycle_size":           textField(RoutingSignals.CycleSize),
	"scout.cycle_size":            textField(func(s RoutingSignals) string { return s.Scout.CycleSizeEstimate }),
	config.SignalDeliverableKind:  textField(RoutingSignals.DeliverableKind),
	config.SignalGoalType:         typedOrGenericField(func(s RoutingSignals) string { return s.Scout.GoalType }),
	"scout.deliverable_kind":      typedOrGenericField(func(s RoutingSignals) string { return s.Scout.DeliverableKind }),
	"triage.deliverable_kind":     typedOrGenericField(func(s RoutingSignals) string { return s.Triage.DeliverableKind }),
	"triage.unified_size":         textField(func(s RoutingSignals) string { return s.Triage.UnifiedSize }),
	"triage.unified_member_count": numberField(func(s RoutingSignals) float64 { return float64(s.Triage.UnifiedMemberCount) }),
	"scout.item_count":            numberField(func(s RoutingSignals) float64 { return float64(s.Scout.ItemCount) }),
	"scout.carryover_count":       numberField(func(s RoutingSignals) float64 { return float64(s.Scout.CarryoverCount) }),
	"scout.backlog_size":          numberField(func(s RoutingSignals) float64 { return float64(s.Scout.BacklogSize) }),
	"build.acs_red":               numberField(func(s RoutingSignals) float64 { return float64(s.Build.ACSRed) }),
	"build.acs_green":             numberField(func(s RoutingSignals) float64 { return float64(s.Build.ACSGreen) }),
	"build.acs_regression":        numberField(func(s RoutingSignals) float64 { return float64(s.Build.ACSRegression) }),
	"build.files_touched":         numberField(func(s RoutingSignals) float64 { return float64(s.Build.FilesTouched) }),
	"build.diff_loc":              numberField(func(s RoutingSignals) float64 { return float64(s.Build.DiffLOC) }),
	"build.severity_max":          numberField(func(s RoutingSignals) float64 { return float64(s.Build.SeverityMax) }),
	"build.verdict":               textField(func(s RoutingSignals) string { return s.Build.Verdict }),
	"audit.confidence":            numberField(func(s RoutingSignals) float64 { return s.Audit.Confidence }),
	"audit.red_count":             numberField(func(s RoutingSignals) float64 { return float64(s.Audit.RedCount) }),
	"audit.verdict":               textField(func(s RoutingSignals) string { return s.Audit.Verdict }),
	"ship.class":                  textField(func(s RoutingSignals) string { return s.Ship.Class }),
	"run_dir.artifact_bytes":      numberField(func(s RoutingSignals) float64 { return float64(s.RunDir.ArtifactBytes) }),
}

func resolveField(sig RoutingSignals, field string) (float64, bool, string, bool) {
	if resolve, ok := typedFields[field]; ok {
		return resolve(sig, field)
	}
	return resolveGeneric(sig, field)
}

func HarnessProduces(field string) bool {
	if _, ok := typedFields[field]; ok {
		return true
	}
	phase, suffix, ok := strings.Cut(field, ".")
	return ok && phase != "" && (suffix == failureClassSignal || suffix == defectCountSignal)
}

func resolveTypedOrGeneric(sig RoutingSignals, field, typed string) (float64, bool, string, bool) {
	if typed != "" {
		return 0, false, typed, true
	}
	return resolveGeneric(sig, field)
}

func resolveGeneric(sig RoutingSignals, field string) (float64, bool, string, bool) {
	v, ok := sig.GenericValue(field)
	if !ok {
		return 0, false, "", false
	}
	switch t := v.(type) {
	case float64:
		return t, true, "", true
	case int:
		return float64(t), true, "", true
	case string:
		return 0, false, t, true
	case bool:
		if t {
			return 0, false, "true", true
		}
		return 0, false, "false", true
	default:
		return 0, false, "", true
	}
}

func coerceNum(field string, v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		if field == "build.severity_max" {
			return float64(ParseSeverity(t)), true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func coerceStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
