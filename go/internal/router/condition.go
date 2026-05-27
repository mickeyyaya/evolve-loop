package router

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// evalCondition is the Specification evaluator: does a single declarative
// trigger clause hold against the digested signals? Field paths reference
// OBJECTIVE handoff values only (never an LLM self-score). Unknown fields are
// false (fail-safe: an unrecognized trigger never fires).
func evalCondition(sig RoutingSignals, c config.Condition) bool {
	num, isNum, str := resolveField(sig, c.Field)
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
// isNumeric, string value). Numeric and string forms are mutually exclusive.
func resolveField(sig RoutingSignals, field string) (float64, bool, string) {
	switch field {
	case "cycle_size", "triage.cycle_size":
		return 0, false, sig.CycleSize()
	case "scout.cycle_size":
		return 0, false, sig.Scout.CycleSizeEstimate
	case "scout.item_count":
		return float64(sig.Scout.ItemCount), true, ""
	case "scout.carryover_count":
		return float64(sig.Scout.CarryoverCount), true, ""
	case "build.acs_red":
		return float64(sig.Build.ACSRed), true, ""
	case "build.acs_green":
		return float64(sig.Build.ACSGreen), true, ""
	case "build.acs_regression":
		return float64(sig.Build.ACSRegression), true, ""
	case "build.files_touched":
		return float64(sig.Build.FilesTouched), true, ""
	case "build.severity_max":
		return float64(sig.Build.SeverityMax), true, ""
	case "build.verdict":
		return 0, false, sig.Build.Verdict
	case "audit.confidence":
		return sig.Audit.Confidence, true, ""
	case "audit.red_count":
		return float64(sig.Audit.RedCount), true, ""
	case "audit.verdict":
		return 0, false, sig.Audit.Verdict
	default:
		// TODO(Stage 2): fall through to sig.Generic so user-phase signals
		// (the uniform signal plane) are routable. Until then an unknown field
		// is fail-safe false (an unrecognized trigger never fires).
		return 0, false, ""
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
