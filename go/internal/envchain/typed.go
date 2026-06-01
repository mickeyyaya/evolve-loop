package envchain

import (
	"strconv"
	"strings"
)

// typed.go layers typed scalar getters over the same four-tier precedence
// chain as Resolve (reqEnv > os.Getenv > profile > default). Before this,
// every read site re-implemented "look up the string, strconv it, fall back
// on empty/parse-error, bounds-check" — the boilerplate drifted (inconsistent
// bool idioms, copy-pasted clamps). These helpers are the one place that
// boilerplate lives.
//
// All getters consult only the env tiers (reqEnv then os.Getenv); the profile
// and default tiers of the raw Resolve are unused here because a typed knob's
// fallback is its typed default argument, not a string.

// Int returns key parsed as an int, or def when the value is unset, empty, or
// not a valid integer. It mirrors the historical `strconv.Atoi(env[key])` with
// fall-back-to-default exactly (no whitespace trimming), so swapping a call
// site to Int is behaviour-preserving.
func Int(key string, reqEnv map[string]string, def int) int {
	raw := Resolve(key, reqEnv, "", "")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// IntMin returns key parsed as an int when it is >= min; otherwise def. A
// value below min is treated as invalid input (→ def), NOT clamped up to min —
// this matches the existing "num > 0 else fall back" guard used by the
// phase-latency ceilings. Unset / empty / unparseable also → def.
//
// Callers are expected to pass def >= min (every call site does); if def < min
// the below-min fallback returns a def that is itself below min, by design —
// the helper does not second-guess the caller's chosen default.
func IntMin(key string, reqEnv map[string]string, def, min int) int {
	n := Int(key, reqEnv, def)
	if n < min {
		return def
	}
	return n
}

// Bool parses key as a boolean over the env chain (reqEnv > os.Getenv).
// Truthy: "1", "true", "yes", "on". Falsy: "0", "false", "no", "off".
// Comparison is case-insensitive. Unset, empty, or any unrecognized value
// returns def.
//
// This is the canonical reader for both styles of flag the codebase grew:
//   - default-off "enable" flags (read as `== "1"`)  → Bool(key, env, false)
//   - default-on  flags (read as `!= "0"`)           → Bool(key, env, true)
//   - inverse "*_DISABLE" flags                       → Bool(key, env, defDisabled)
//
// Bool recognizes a superset of the documented "1"/"0" tokens (it also accepts
// true/false/yes/no/on/off); for the "1"/"0" values every legacy call site
// used, it returns the identical result, so migrations are behaviour-preserving.
//
// Bool consults os.Getenv via Resolve; for an already-resolved string — e.g. a
// value read from a deliberately-frozen per-cycle env snapshot that MUST NOT
// fall back to the live process env — use BoolValue instead.
func Bool(key string, reqEnv map[string]string, def bool) bool {
	return BoolValue(Resolve(key, reqEnv, "", ""), def)
}

// BoolValue interprets an already-resolved string with the same truthy/falsy
// vocabulary as Bool, returning def for empty or unrecognized input. It does
// NO env lookup, so it is the correct tool for map-only reads (frozen
// snapshots) that must not leak into the live process env.
func BoolValue(raw string, def bool) bool {
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
