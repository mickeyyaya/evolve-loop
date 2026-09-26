package main

import (
	"math"
	"strconv"
	"strings"
)

// isRemovedBudgetFlag reports whether name is a retired cost-budget flag; cost
// is display-only telemetry, never a cap input.
func isRemovedBudgetFlag(name string) bool {
	switch name {
	case "budget-usd", "budget", "batch-cap-usd":
		return true
	default:
		return false
	}
}

// stripRemovedBudgetFlags removes every removed budget flag and its value in any
// form flag.Parse accepts, warning once, so old scripts get a notice rather
// than a "flag provided but not defined" abort.
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
		// The flags are float-valued, so a numeric next token (even "-1") is the
		// value; a non-numeric one such as "--cycles" is a real flag.
		if !hasEqValue && i+1 < len(args) && isFloatValue(args[i+1]) {
			i++
		}
	}
	return out
}

// removedBudgetFlagName returns the canonical name of a removed budget flag, or
// "", and whether its value was attached with "=".
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

// isFloatValue reports whether tok is a finite number; "Inf" and "NaN" are
// rejected so a goal word of that form is never eaten as a flag value.
func isFloatValue(tok string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(tok), 64)
	return err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}
