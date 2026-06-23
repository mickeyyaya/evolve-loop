package router

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
)

// evalCondition is the Specification evaluator: does a single declarative
// trigger clause hold against the digested signals? Field paths reference
// OBJECTIVE handoff values only (never an LLM self-score). Unknown fields are
// false (fail-safe: an unrecognized trigger never fires).
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

// evalCondRule evaluates a conditional-mandatory rule (string value form).
func evalCondRule(sig RoutingSignals, r config.CondRule) bool {
	return evalCondition(sig, config.Condition{Field: r.Field, Op: r.Op, Value: r.Value})
}

// resolveField maps a field path to its signal value. Returns (numeric value,
// isNumeric, string value, isPresent). Numeric and string forms are mutually exclusive.
func resolveField(sig RoutingSignals, field string) (float64, bool, string, bool) {
	switch field {
	case "cycle_size", "triage.cycle_size":
		return 0, false, sig.CycleSize(), true
	case "scout.cycle_size":
		return 0, false, sig.Scout.CycleSizeEstimate, true
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
		// Fall through to the uniform signal plane: a field not covered by the
		// typed structs above is resolved from sig.Generic, so a user-defined
		// phase's emitted signal is routable. Absent → fail-safe false (an
		// unrecognized trigger never fires).
		return resolveGeneric(sig, field)
	}
}

// resolveGeneric resolves field from the namespaced generic signal bus. JSON
// numbers arrive as float64; strings stay strings; bools render as
// "true"/"false" so eq/ne comparisons work. Anything else is fail-safe false.
func resolveGeneric(sig RoutingSignals, field string) (float64, bool, string, bool) {
	v, ok := sig.GenericValue(field)
	if !ok {
		return 0, false, "", false
	}
	switch t := v.(type) {
	case float64:
		return t, true, "", true
	case int:
		return float64(t), true, "", true // in-process assignment (encoding/json always emits float64)
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

// coerceNum converts a condition value to float64. For severity fields a string
// value is mapped through the severity ordinal (so value:"HIGH" works).
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
