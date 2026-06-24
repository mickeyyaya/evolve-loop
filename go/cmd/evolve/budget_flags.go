package main

import (
	"math"
	"strconv"
	"strings"
)

// isRemovedBudgetFlag reports whether name is one of the retired cost-budget
// flags (--budget-usd / --budget / --batch-cap-usd). They were long-since
// reduced to deprecated no-ops; they are now removed from the parameter surface
// entirely. Per-cycle token cost is tracked accurately across LLM CLIs as
// display-only telemetry (total_cost_usd / per-phase cost_usd) — it is NOT a cap
// input. Bound a run with --cycles N (or let the advisor decide) instead.
// A stateless switch (not a package-level map) keeps the set immutable.
func isRemovedBudgetFlag(name string) bool {
	switch name {
	case "budget-usd", "budget", "batch-cap-usd":
		return true
	default:
		return false
	}
}

// stripRemovedBudgetFlags removes every occurrence of a removed cost-budget flag
// (and its value) from args, emitting a single consolidated WARN via warn if any
// were present. Removing the flags from the FlagSet means flag.Parse would reject
// them ("flag provided but not defined") and abort the run; stripping them here
// first keeps old scripts/CI working — they get a deprecation notice, not a crash.
//
// It handles every form flag.Parse accepts for a value-bearing flag:
// -flag, --flag, -flag=v, --flag=v, and the space-separated "-flag v".
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
		// Space-separated form ("--budget-usd 5") carries its value in the next
		// token — drop that too. The removed flags are all float-valued, so a
		// following token that parses as a float is the value (this also catches a
		// negative like "-1", which a naive leading-dash check would mistake for
		// another flag); a non-numeric token (e.g. "--cycles") is left in place.
		if !hasEqValue && i+1 < len(args) && isFloatValue(args[i+1]) {
			i++
		}
	}
	return out
}

// removedBudgetFlagName reports the canonical flag name if arg is one of the
// removed budget flags in -flag / --flag / -flag=v form, and whether the value
// was attached via "=value" (so the caller knows not to also consume the next
// token). Returns "" for anything that is not a removed budget flag.
func removedBudgetFlagName(arg string) (name string, hasEqValue bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	s := arg[1:]
	if s[0] == '-' { // accept both -flag and --flag, like the flag package
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

// isFloatValue reports whether tok is a bare, finite numeric value (e.g. "5",
// "7.50", "-1") — i.e. the space-separated value of a removed float-valued
// budget flag, as opposed to a following flag token like "--cycles". Non-finite
// inputs ParseFloat would otherwise accept ("Inf", "NaN") are rejected so a
// pathological positional goal of that form is never mistaken for a value.
func isFloatValue(tok string) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(tok), 64)
	return err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}
