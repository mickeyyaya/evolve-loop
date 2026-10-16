package main

import (
	"math"
	"strconv"
	"strings"
)

func isRemovedBudgetFlag(name string) bool {
	switch name {
	case "budget-usd", "budget", "batch-cap-usd":
		return true
	default:
		return false
	}
}

func stripRemovedBudgetFlags(args []string, warn func(string)) []string {
	out := make([]string, 0, len(args))
	warned := false
	for i := 0; i < len(args); i++ {
		name, hasEqValue := removedBudgetFlagName(args[i])
		if name == "" {
			out = append(out, args[i])
			continue
		}
		if !warned {
			warn("--budget-usd/--budget/--batch-cap-usd are removed; per-cycle token cost is " +
				"display-only telemetry now, not a cap input. Use --cycles N to bound a run " +
				"(or omit it and let the advisor decide); ignoring.")
			warned = true
		}
		if !hasEqValue && i+1 < len(args) && isFloatValue(args[i+1]) {
			i++
		}
	}
	return out
}

func removedBudgetFlagName(arg string) (name string, hasEqValue bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	s := arg[1:]
	if s[0] == '-' {
		s = s[1:]
	}
	if eq := strings.IndexByte(s, '='); eq >= 0 {
		if isRemovedBudgetFlag(s[:eq]) {
			return s[:eq], true
		}
		return "", false
	}
	if isRemovedBudgetFlag(s) {
		return s, false
	}
	return "", false
}

func isFloatValue(tok string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(tok), 64)
	return err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}
