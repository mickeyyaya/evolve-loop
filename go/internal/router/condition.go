package router

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// evalCondition reports whether one trigger clause holds against the signals; an absent field never holds.
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

// evalCondRule evaluates a conditional-mandatory rule: its head clause AND every clause in r.And.
func evalCondRule(sig RoutingSignals, r config.CondRule) bool {
	for _, c := range r.Clauses() {
		if !evalCondition(sig, config.Condition{Field: c.Field, Op: c.Op, Value: c.Value}) {
			return false
		}
	}
	return true
}

// resolveField returns (number, isNumber, string, isPresent) for a field path. A typed field is
// present even at its zero value, so `cycle_size != trivial` holds before any handoff.
func resolveField(sig RoutingSignals, field string) (float64, bool, string, bool) {
	switch field {
	case "cycle_size", "triage.cycle_size":
		return 0, false, sig.CycleSize(), true
	case "scout.cycle_size":
		return 0, false, sig.Scout.CycleSizeEstimate, true
	case config.SignalDeliverableKind:
		return 0, false, sig.DeliverableKind(), true
	case config.SignalGoalType:
		return resolveTypedOrGeneric(sig, field, sig.Scout.GoalType)
	case "scout.deliverable_kind":
		return resolveTypedOrGeneric(sig, field, sig.Scout.DeliverableKind)
	case "triage.deliverable_kind":
		return resolveTypedOrGeneric(sig, field, sig.Triage.DeliverableKind)
	case "triage.unified_size":
		return 0, false, sig.Triage.UnifiedSize, true
	case "triage.unified_member_count":
		return float64(sig.Triage.UnifiedMemberCount), true, "", true
	case "scout.item_count":
		return float64(sig.Scout.ItemCount), true, "", true
	case "scout.carryover_count":
		return float64(sig.Scout.CarryoverCount), true, "", true
	case "scout.backlog_size":
		return float64(sig.Scout.BacklogSize), true, "", true
	case "build.acs_red":
		return float64(sig.Build.ACSRed), true, "", true
	case "build.acs_green":
		return float64(sig.Build.ACSGreen), true, "", true
	case "build.acs_regression":
		return float64(sig.Build.ACSRegression), true, "", true
	case "build.files_touched":
		return float64(sig.Build.FilesTouched), true, "", true
	case "build.diff_loc":
		return float64(sig.Build.DiffLOC), true, "", true
	case "build.severity_max":
		return float64(sig.Build.SeverityMax), true, "", true
	case "build.verdict":
		return 0, false, sig.Build.Verdict, true
	case "audit.confidence":
		return sig.Audit.Confidence, true, "", true
	case "audit.red_count":
		return float64(sig.Audit.RedCount), true, "", true
	case "audit.verdict":
		return 0, false, sig.Audit.Verdict, true
	default:
		return resolveGeneric(sig, field)
	}
}

// resolveTypedOrGeneric returns the declared typed value, else the generic signal. An undeclared
// value stays absent, so an `ne` trigger never fires on a cycle that declared nothing.
func resolveTypedOrGeneric(sig RoutingSignals, field, typed string) (float64, bool, string, bool) {
	if typed != "" {
		return 0, false, typed, true
	}
	return resolveGeneric(sig, field)
}

// resolveGeneric resolves field from the generic plane; bools render as "true"/"false" so eq and ne work.
func resolveGeneric(sig RoutingSignals, field string) (float64, bool, string, bool) {
	v, ok := sig.GenericValue(field)
	if !ok {
		return 0, false, "", false
	}
	switch t := v.(type) {
	case float64:
		return t, true, "", true
	case int:
		return float64(t), true, "", true // set in process; encoding/json always yields float64
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

// coerceNum converts a condition value to float64; a severity field accepts a word such as "HIGH".
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
