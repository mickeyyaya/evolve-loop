package bridge

import (
	"fmt"
	"io"
	"strings"
)

// exhaustion_drift.go — a fail-loud DRIFT alarm for the exhausted_regex.
//
// Exhaustion detection keys off a regex that must track a provider's UI wording,
// which changes without notice. The per-model "You've reached your <Model> limit"
// wording (Claude Code v2.1.212) silently stopped matching the legacy pattern and
// 8 audit cycles burned as generic exit-81 artifact timeouts before an operator
// hand-read a pane and spotted it. A fail-OPEN detector (a regex miss degrades to
// "not exhausted") is indistinguishable from "all healthy" unless something
// watches for the miss. This is that watcher: it converts the NEXT such drift
// from an 8-cycle silent burn into a single loud line.

// warnExhaustionRegexDrift runs ONLY on an already-failed exit-81 teardown. When
// the final pane matches the manifest's BROAD drift heuristic
// (controls.usage.drift_probe_regex) but the REAL exhausted_regex did NOT match
// the same pane, the wall wording likely drifted ahead of exhausted_regex — emit
// a loud, greppable diagnostic that names the fix. The broad probe is
// DIAGNOSTIC-ONLY: it never drives the fast-fail (it is deliberately loose and
// would false-fail a working agent); it only annotates a failure that already
// happened, so the verdict is unchanged.
func warnExhaustionRegexDrift(w io.Writer, pfx, cli, pane, exhaustedRegex string) {
	if w == nil || strings.TrimSpace(pane) == "" {
		return
	}
	probe := manifestDriftProbePattern(cli)
	if probe == "" {
		return // no heuristic configured for this CLI — drift alarm off (fail-open)
	}
	// Fire only when the loose heuristic sees a wall-shaped signal AND the real
	// pattern missed it: that gap is exactly the drift signature. matchExhausted
	// (usageclassify.go) is the one maintained "compile + fail-open match"
	// helper in this package — reused here rather than duplicated.
	if matchExhausted(probe, pane) && !matchExhausted(exhaustedRegex, pane) {
		fmt.Fprintf(w, "%s POSSIBLE EXHAUSTION-REGEX DRIFT: the teardown pane matches a broad quota-wall heuristic but %s's controls.usage.exhausted_regex did not — the wall wording may have changed; update the exhausted_regex (diagnostic only, this exit-81 verdict is unchanged).\n", pfx, cli)
	}
}

// manifestDriftProbePattern loads cli's embedded manifest and returns its
// controls.usage.drift_probe_regex, or "" when absent/unloadable (drift alarm off).
func manifestDriftProbePattern(cli string) string {
	m, err := LoadManifest(cli)
	if err != nil {
		return ""
	}
	if spec, ok := m.Control("usage"); ok {
		return spec.DriftProbeRegex
	}
	return ""
}
